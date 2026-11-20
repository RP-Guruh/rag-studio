//go:build integration
// +build integration

package projects

import (
	"context"
	"encoding/base64"
	"os"
	"testing"
	"time"

	"rag_project/internal/settings"
)

// TestDocumentLifecycleIntegration exercises the full local-file +
// PostgreSQL metadata flow against the configured development database.
// Run with: go test -tags integration ./internal/projects -run TestDocumentLifecycleIntegration -v
func TestDocumentLifecycleIntegration(t *testing.T) {
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

	db, err := NewStoreFromSettings(cfg.Database, password)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	project, err := db.CreateProject(ctx, "__upload_integration_test__", "temporary test project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer func() {
		_ = db.DeleteProject(ctx, project.ID)
	}()

	content := []byte("hello rag studio")
	documents, err := db.AddDocuments(ctx, project.ID, []DocumentInput{{
		Filename:      "hello.txt",
		MimeType:      "text/plain",
		ContentBase64: base64.StdEncoding.EncodeToString(content),
	}})
	if err != nil {
		t.Fatalf("add document: %v", err)
	}
	if len(documents) != 1 {
		t.Fatalf("got %d documents, want 1", len(documents))
	}

	doc := documents[0]
	if doc.SHA256 != Checksum(content) {
		t.Errorf("sha256 = %q, want %q", doc.SHA256, Checksum(content))
	}
	if doc.SizeBytes != int64(len(content)) {
		t.Errorf("size = %d, want %d", doc.SizeBytes, len(content))
	}
	if _, err := os.Stat(doc.StoragePath); err != nil {
		t.Errorf("stored file missing: %v", err)
	}

	// Uploading the same bytes again must not create a duplicate row.
	again, err := db.AddDocuments(ctx, project.ID, []DocumentInput{{
		Filename:      "hello.txt",
		MimeType:      "text/plain",
		ContentBase64: base64.StdEncoding.EncodeToString(content),
	}})
	if err != nil {
		t.Fatalf("re-add document: %v", err)
	}
	if len(again) != 1 || again[0].ID != doc.ID {
		t.Errorf("duplicate upload returned %+v, want the existing document", again)
	}

	loaded, err := db.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if loaded == nil || len(loaded.Documents) != 1 || loaded.Documents[0].ID != doc.ID {
		t.Errorf("GetProject documents = %+v, want the uploaded document", loaded)
	}

	if err := db.DeleteDocument(ctx, project.ID, doc.ID); err != nil {
		t.Fatalf("delete document: %v", err)
	}
	if _, err := os.Stat(doc.StoragePath); !os.IsNotExist(err) {
		t.Errorf("file still exists after delete: %v", err)
	}
}
