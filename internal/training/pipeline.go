package training

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/miku/doclingclient"

	"rag_project/internal/docling"
	"rag_project/internal/embedding"
	"rag_project/internal/projects"
)

const (
	dbWriteTimeout    = 10 * time.Second
	previewChunkCount = 3
	previewTextRunes  = 300
)

// Pipeline chunks a document through Docling, embeds every chunk and persists
// both the vectors and the document status.
type Pipeline struct {
	Store    *projects.Store
	Chunker  docling.Chunker
	Embedder embedding.Embedder
	Timeout  time.Duration
	Logger   *slog.Logger
}

// NewPipeline builds a Pipeline for one training job. The timeout must come
// from Settings → Document processing; no fallback is applied.
func NewPipeline(store *projects.Store, chunker docling.Chunker, embedder embedding.Embedder, timeout time.Duration, logger *slog.Logger) (*Pipeline, error) {
	if store == nil {
		return nil, fmt.Errorf("training pipeline requires a database store")
	}
	if chunker == nil {
		return nil, fmt.Errorf("training pipeline requires a chunker")
	}
	if embedder == nil {
		return nil, fmt.Errorf("training pipeline requires an embedder")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("document timeout is not configured. Open Settings → Document processing")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Pipeline{Store: store, Chunker: chunker, Embedder: embedder, Timeout: timeout, Logger: logger}, nil
}

// Process chunks one document, stores its embeddings and records the outcome
// in PostgreSQL. Any failure after the "processing" mark is recorded as a
// failed document so a later re-index can retry it.
func (p *Pipeline) Process(ctx context.Context, doc projects.Document) (int, error) {
	if err := p.mark(ctx, func(dbCtx context.Context) error {
		return p.Store.MarkDocumentProcessing(dbCtx, doc.ProjectID, doc.ID)
	}); err != nil {
		return 0, err
	}

	result, err := p.chunk(ctx, doc)
	if err != nil {
		return 0, p.fail(ctx, doc, err)
	}

	chunks := mapChunks(result.Chunks, doc.ProjectID, doc.ID)
	if len(chunks) > 0 {
		texts := make([]string, len(chunks))
		for i := range chunks {
			texts[i] = chunks[i].Content
		}
		embedCtx, cancel := context.WithTimeout(ctx, p.Timeout)
		vectors, err := p.Embedder.Embed(embedCtx, texts)
		cancel()
		if err != nil {
			return 0, p.fail(ctx, doc, err)
		}
		for i := range chunks {
			chunks[i].Embedding = vectors[i]
		}
	}

	if err := p.mark(ctx, func(dbCtx context.Context) error {
		return p.Store.ReplaceDocumentChunks(dbCtx, doc.ProjectID, doc.ID, chunks)
	}); err != nil {
		return 0, p.fail(ctx, doc, err)
	}

	if err := p.mark(ctx, func(dbCtx context.Context) error {
		return p.Store.MarkDocumentIndexed(dbCtx, doc.ProjectID, doc.ID, p.Embedder.Model(), p.Embedder.Dimension(), len(chunks))
	}); err != nil {
		return 0, p.fail(ctx, doc, err)
	}

	p.logResult(doc, result, len(chunks))
	return len(chunks), nil
}

func (p *Pipeline) chunk(ctx context.Context, doc projects.Document) (*docling.Result, error) {
	chunkCtx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()
	return p.Chunker.Chunk(chunkCtx, doc.StoragePath, doc.Filename)
}

func (p *Pipeline) fail(ctx context.Context, doc projects.Document, cause error) error {
	if ctx.Err() == nil {
		message := truncate(cause.Error(), 500)
		if err := p.mark(ctx, func(dbCtx context.Context) error {
			return p.Store.MarkDocumentFailed(dbCtx, doc.ProjectID, doc.ID, message)
		}); err != nil {
			p.Logger.Warn("could not record failed document", "document", doc.Filename, "error", err)
		}
	}
	p.Logger.Warn("document pipeline failed",
		"project", doc.ProjectID,
		"document", doc.Filename,
		"error", cause,
	)
	return cause
}

// mapChunks copies the Docling result into persistable chunks, preserving the
// location and metadata needed to show sources in the Playground.
func mapChunks(items []doclingclient.Chunk, projectID, documentID string) []projects.Chunk {
	out := make([]projects.Chunk, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		page := 0
		if len(item.PageNumbers) > 0 {
			page = item.PageNumbers[0]
		}
		tokens := 0
		if item.NumTokens != nil {
			tokens = *item.NumTokens
		}
		metadata := map[string]any{}
		if len(item.Headings) > 0 {
			metadata["headings"] = item.Headings
		}
		if len(item.Captions) > 0 {
			metadata["captions"] = item.Captions
		}
		if len(item.DocItems) > 0 {
			metadata["docItems"] = item.DocItems
		}
		if item.RawText != "" {
			metadata["rawText"] = item.RawText
		}
		for key, value := range item.Metadata {
			metadata[key] = value
		}
		out = append(out, projects.Chunk{
			DocumentID: documentID,
			ProjectID:  projectID,
			ChunkIndex: item.ChunkIndex,
			Content:    text,
			TokenCount: tokens,
			PageNumber: page,
			Metadata:   metadata,
		})
	}
	return out
}

// Close releases the database pool owned by this job.
func (p *Pipeline) Close() error {
	if p == nil || p.Store == nil {
		return nil
	}
	p.Store.Close()
	return nil
}

func (p *Pipeline) mark(_ context.Context, fn func(context.Context) error) error {
	dbCtx, cancel := context.WithTimeout(context.Background(), dbWriteTimeout)
	defer cancel()
	if err := fn(dbCtx); err != nil {
		return fmt.Errorf("update document status: %w", err)
	}
	return nil
}

func (p *Pipeline) logResult(doc projects.Document, result *docling.Result, chunks int) {
	p.Logger.Info("document indexed",
		"project", doc.ProjectID,
		"document", doc.Filename,
		"chunks", chunks,
		"processing_time_seconds", result.ProcessingTime,
	)

	limit := previewChunkCount
	if len(result.Chunks) < limit {
		limit = len(result.Chunks)
	}
	for i := 0; i < limit; i++ {
		chunk := result.Chunks[i]
		tokens := 0
		if chunk.NumTokens != nil {
			tokens = *chunk.NumTokens
		}
		p.Logger.Info("docling chunk preview",
			"document", doc.Filename,
			"index", chunk.ChunkIndex,
			"tokens", tokens,
			"headings", chunk.Headings,
			"pages", chunk.PageNumbers,
			"text", truncate(chunk.Text, previewTextRunes),
		)
	}
}

func truncate(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}
