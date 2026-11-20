//go:build integration
// +build integration

package service

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"rag_project/internal/projects"
	"rag_project/internal/settings"
	"rag_project/internal/training"
)

// TestTrainProjectIntegration exercises the Wails-facing training entrypoint
// end to end. Run with:
//
//	go test -tags integration ./internal/service -run TestTrainProjectIntegration -v
func TestTrainProjectIntegration(t *testing.T) {
	repo, err := settings.OpenDefault()
	if err != nil {
		t.Fatalf("open settings: %v", err)
	}
	defer repo.Close()

	cfg, err := repo.Get()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	password, err := repo.DatabasePassword()
	if err != nil {
		t.Fatalf("load database password: %v", err)
	}

	db, err := projects.NewStoreFromSettings(cfg.Database, password)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := db.EnsureDocumentStatusConstraint(ctx); err != nil {
		t.Fatalf("ensure status constraint: %v", err)
	}

	project, err := db.CreateProject(ctx, "__train_service_integration_test__", "temporary test project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer func() {
		_ = db.DeleteProject(context.Background(), project.ID)
	}()

	content := []byte("# Queue notes\n\nWorkers process one document at a time.\n")
	docs, err := db.AddDocuments(ctx, project.ID, []projects.DocumentInput{{
		Filename:      "queue-notes.md",
		MimeType:      "text/markdown",
		ContentBase64: base64.StdEncoding.EncodeToString(content),
	}})
	if err != nil {
		t.Fatalf("add document: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents, want 1", len(docs))
	}

	service := NewProjectService(repo, training.NewManager(2))
	progress, err := service.TrainProject(project.ID)
	if err != nil {
		t.Fatalf("train project: %v", err)
	}
	if progress == nil {
		t.Fatal("expected initial progress")
	}

	deadline := time.Now().Add(2 * time.Minute)
	for {
		status, err := service.GetTrainingStatus(project.ID)
		if err != nil {
			t.Fatalf("get training status: %v", err)
		}
		if status != nil && !status.Running() {
			if status.Succeeded != 1 || status.Failed != 0 {
				t.Fatalf("unexpected training result: %+v", status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("training did not finish in time: %+v", status)
		}
		time.Sleep(250 * time.Millisecond)
	}

	loaded, err := db.GetDocument(ctx, project.ID, docs[0].ID)
	if err != nil {
		t.Fatalf("reload document: %v", err)
	}
	if loaded == nil || loaded.Status != "chunked" {
		t.Fatalf("document = %+v, want status chunked", loaded)
	}
	if loaded.ChunkCount == 0 {
		t.Error("expected chunk_count > 0")
	}
}
