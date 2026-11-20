// Package docling adapts the docling-serve HTTP client to the training
// pipeline. It owns all knowledge of github.com/miku/doclingclient.
package docling

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/miku/doclingclient"

	"rag_project/internal/settings"
)

// Result summarises a single chunking run.
type Result struct {
	Chunks         []doclingclient.Chunk
	ProcessingTime float64
}

// ChunkCount returns the number of chunks produced.
func (r *Result) ChunkCount() int {
	if r == nil {
		return 0
	}
	return len(r.Chunks)
}

// ChunkOptions controls the hybrid chunker. They are read from the embedding
// model settings so chunk boundaries match the model's tokenizer.
type ChunkOptions struct {
	MaxTokens int
	Tokenizer string
}

func (o ChunkOptions) validate() error {
	if o.MaxTokens <= 0 {
		return errors.New("chunk max tokens is not configured. Open Settings → Embedding Model and save a value")
	}
	if strings.TrimSpace(o.Tokenizer) == "" {
		return errors.New("chunker tokenizer is not configured. Open Settings → Embedding Model and save a tokenizer")
	}
	return nil
}

// Chunker turns a local document file into chunks.
type Chunker interface {
	Chunk(ctx context.Context, path, filename string) (*Result, error)
}

// ClientChunker calls docling-serve through doclingclient.
type ClientChunker struct {
	client *doclingclient.Client
	opts   ChunkOptions
}

// NewChunker builds a Chunker from the Document Processing and Embedding Model
// settings. It never substitutes defaults: every value must be configured.
func NewChunker(cfg settings.DoclingConnection, apiKey string, chunkOptions ChunkOptions) (*ClientChunker, error) {
	if err := chunkOptions.validate(); err != nil {
		return nil, err
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, errors.New("Docling base URL is not configured. Open Settings → Document processing")
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		return nil, errors.New("Docling timeout is not configured. Open Settings → Document processing")
	}

	opts := []doclingclient.Option{doclingclient.WithTimeout(timeout)}
	if apiKey != "" {
		opts = append(opts, doclingclient.WithAPIKey(apiKey))
	}

	return &ClientChunker{
		client: doclingclient.New(baseURL, opts...),
		opts:   chunkOptions,
	}, nil
}

// Chunk uploads one file to docling-serve and returns its hybrid chunks.
func (c *ClientChunker) Chunk(ctx context.Context, path, filename string) (*Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	resp, err := c.client.ChunkHybridFile(ctx, doclingclient.ChunkHybridFileRequest{
		// Use the original filename so chunk metadata is readable instead of
		// exposing the checksum-based name used on disk.
		Files: []doclingclient.File{doclingclient.FileReader{
			Filename: filename,
			Reader:   file,
		}},
		ConvertOptions: doclingclient.ConvertOptions{
			DoOCR: doclingclient.Ptr(true),
		},
		ChunkingOptions: doclingclient.HybridChunkerOptions{
			MaxTokens:      doclingclient.Ptr(c.opts.MaxTokens),
			Tokenizer:      c.opts.Tokenizer,
			MergePeers:     doclingclient.Ptr(true),
			IncludeRawText: doclingclient.Ptr(true),
		},
	})
	if err != nil {
		return nil, err
	}

	return &Result{Chunks: resp.Chunks, ProcessingTime: resp.ProcessingTime}, nil
}
