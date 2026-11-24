//go:build integration
// +build integration

package training

import (
	"context"
	"encoding/base64"
	"log/slog"
	"os"
	"testing"
	"time"

	"rag_project/internal/docling"
	"rag_project/internal/embedding"
	"rag_project/internal/projects"
	"rag_project/internal/settings"
)

// TestTrainingPipelineIntegration chunks a real document through the configured
// Docling server and verifies the persisted status. Run with:
//
//	go test -tags integration ./internal/training -run TestTrainingPipelineIntegration -v
func TestTrainingPipelineIntegration(t *testing.T) {
	store, err := settings.OpenDefault()
	if err != nil {
		t.Fatalf("open settings: %v", err)
	}
	defer store.Close()

	cfg, err := store.Get()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	password, err := store.DatabasePassword()
	if err != nil {
		t.Fatalf("load database password: %v", err)
	}
	apiKey, err := store.DoclingAPIKey()
	if err != nil {
		t.Fatalf("load Docling API key: %v", err)
	}

	db, err := projects.NewStoreFromSettings(cfg.Database, password)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := db.EnsureDocumentStatusConstraint(ctx); err != nil {
		t.Fatalf("ensure status constraint: %v", err)
	}
	if err := db.EnsureChunksVectorDimension(ctx, cfg.EmbeddingModel.VectorDimension); err != nil {
		t.Fatalf("ensure vector dimension: %v", err)
	}

	project, err := db.CreateProject(ctx, "__training_integration_test__", "temporary test project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer func() {
		_ = db.DeleteProject(context.Background(), project.ID)
	}()

	content := []byte("# Retrieval notes\n\nDocling converts documents into chunks.\n\n" +
		"Chunking keeps headings and paragraphs together for retrieval.\n")
	docs, err := db.AddDocuments(ctx, project.ID, []projects.DocumentInput{{
		Filename:      "retrieval-notes.md",
		MimeType:      "text/markdown",
		ContentBase64: base64.StdEncoding.EncodeToString(content),
	}})
	if err != nil {
		t.Fatalf("add document: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents, want 1", len(docs))
	}

	chunkOptions := docling.ChunkOptions{
		MaxTokens: cfg.EmbeddingModel.ChunkMaxTokens,
		Tokenizer: cfg.EmbeddingModel.Tokenizer,
	}
	chunker, err := docling.NewChunker(cfg.Docling, apiKey, chunkOptions)
	if err != nil {
		t.Fatalf("build chunker: %v", err)
	}
	embeddingKey, err := store.EmbeddingAPIKey()
	if err != nil {
		t.Fatalf("load embedding API key: %v", err)
	}
	embedder, err := embedding.New(cfg.EmbeddingModel, embeddingKey)
	if err != nil {
		t.Fatalf("build embedder: %v", err)
	}
	pipeline, err := NewPipeline(db, chunker, embedder, 90*time.Second, slog.Default())
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	chunks, err := pipeline.Process(ctx, docs[0])
	if err != nil {
		t.Fatalf("process document: %v", err)
	}
	if chunks == 0 {
		t.Fatal("expected at least one chunk from Docling")
	}
	hits, err := db.SearchChunks(ctx, project.ID, make([]float32, cfg.EmbeddingModel.VectorDimension), 5)
	if err != nil {
		t.Fatalf("search chunks: %v", err)
	}
	if len(hits) != chunks {
		t.Errorf("stored chunks = %d, want %d", len(hits), chunks)
	}

	loaded, err := db.GetDocument(ctx, project.ID, docs[0].ID)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if loaded == nil {
		t.Fatal("document disappeared")
	}
	if loaded.Status != "indexed" {
		t.Errorf("status = %q, want indexed", loaded.Status)
	}
	if loaded.ChunkCount != chunks {
		t.Errorf("chunk_count = %d, want %d", loaded.ChunkCount, chunks)
	}
	if _, err := os.Stat(loaded.StoragePath); err != nil {
		t.Errorf("stored file missing: %v", err)
	}
}
