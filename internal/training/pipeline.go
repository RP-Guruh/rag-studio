package training

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"rag_project/internal/docling"
	"rag_project/internal/projects"
)

const (
	dbWriteTimeout    = 10 * time.Second
	previewChunkCount = 3
	previewTextRunes  = 300
)

// Pipeline chunks a document through Docling and persists its status.
type Pipeline struct {
	Store   *projects.Store
	Chunker docling.Chunker
	Timeout time.Duration
	Logger  *slog.Logger
}

// NewPipeline builds a Pipeline for one training job. The timeout must come
// from Settings → Document processing; no fallback is applied.
func NewPipeline(store *projects.Store, chunker docling.Chunker, timeout time.Duration, logger *slog.Logger) (*Pipeline, error) {
	if store == nil {
		return nil, fmt.Errorf("training pipeline requires a database store")
	}
	if chunker == nil {
		return nil, fmt.Errorf("training pipeline requires a chunker")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("document timeout is not configured. Open Settings → Document processing")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Pipeline{Store: store, Chunker: chunker, Timeout: timeout, Logger: logger}, nil
}

// Process chunks one document and records the outcome in PostgreSQL.
func (p *Pipeline) Process(ctx context.Context, doc projects.Document) (int, error) {
	if err := p.mark(ctx, func(dbCtx context.Context) error {
		return p.Store.MarkDocumentProcessing(dbCtx, doc.ProjectID, doc.ID)
	}); err != nil {
		return 0, err
	}

	chunkCtx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	result, err := p.Chunker.Chunk(chunkCtx, doc.StoragePath, doc.Filename)
	if err != nil {
		message := truncate(err.Error(), 500)
		if ctx.Err() == nil {
			_ = p.mark(ctx, func(dbCtx context.Context) error {
				return p.Store.MarkDocumentFailed(dbCtx, doc.ProjectID, doc.ID, message)
			})
		}
		p.Logger.Warn("docling chunk failed",
			"project", doc.ProjectID,
			"document", doc.Filename,
			"error", err,
		)
		return 0, err
	}

	chunks := result.ChunkCount()
	if err := p.mark(ctx, func(dbCtx context.Context) error {
		return p.Store.MarkDocumentChunked(dbCtx, doc.ProjectID, doc.ID, chunks)
	}); err != nil {
		return 0, err
	}

	p.logResult(doc, result, chunks)
	return chunks, nil
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
	p.Logger.Info("docling chunked",
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
