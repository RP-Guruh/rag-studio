package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"rag_project/internal/docling"
	"rag_project/internal/projects"
	"rag_project/internal/settings"
	"rag_project/internal/training"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	trainingEventProgress = "training:progress"
	trainingEventFinished = "training:finished"
)

type ProjectService struct {
	settingsRepo settings.Repository
	trainer      *training.Manager
}

func NewProjectService(settingsRepo settings.Repository, trainer *training.Manager) *ProjectService {
	if trainer == nil {
		trainer = training.NewManager(2)
	}
	return &ProjectService{settingsRepo: settingsRepo, trainer: trainer}
}

func (s *ProjectService) getStore() (*projects.Store, error) {
	settingsData, err := s.settingsRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("load database settings: %w", err)
	}
	password, err := s.settingsRepo.DatabasePassword()
	if err != nil {
		return nil, fmt.Errorf("load database password: %w", err)
	}
	if settingsData.Database.Host == "" || settingsData.Database.DatabaseName == "" {
		return nil, errors.New("database is not configured yet. Please configure and save database settings first.")
	}
	return projects.NewStoreFromSettings(settingsData.Database, password)
}

func (s *ProjectService) ListProjects() ([]projects.Project, error) {
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.ListProjects(ctx)
}

func (s *ProjectService) GetProject(id string) (*projects.Project, error) {
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.GetProject(ctx, id)
}

func (s *ProjectService) CreateProject(name, description string) (*projects.Project, error) {
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.CreateProject(ctx, name, description)
}

func (s *ProjectService) UpdateProject(id, name, description string) (*projects.Project, error) {
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.UpdateProject(ctx, id, name, description)
}

func (s *ProjectService) DeleteProject(id string) error {
	store, err := s.getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.DeleteProject(ctx, id)
}

func (s *ProjectService) UpdatePersona(projectId string, persona projects.PersonaConfig) error {
	store, err := s.getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return store.UpdatePersona(ctx, projectId, persona)
}

// ImportDocuments stores uploaded files on the local disk and records their
// metadata in PostgreSQL.
func (s *ProjectService) ImportDocuments(projectId string, files []projects.DocumentInput) ([]projects.Document, error) {
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	return store.AddDocuments(ctx, projectId, files)
}

// DeleteDocument removes a document from PostgreSQL and deletes its local file.
func (s *ProjectService) DeleteDocument(projectId, documentId string) error {
	store, err := s.getStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return store.DeleteDocument(ctx, projectId, documentId)
}

func (s *ProjectService) MigrateDatabase() error {
	settingsData, err := s.settingsRepo.Get()
	if err != nil {
		return fmt.Errorf("load database settings: %w", err)
	}
	if err := requireDatabase(settingsData.Database); err != nil {
		return err
	}
	if err := requireEmbedding(settingsData.EmbeddingModel); err != nil {
		return err
	}
	password, err := s.settingsRepo.DatabasePassword()
	if err != nil {
		return fmt.Errorf("load database password: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return projects.MigratePostgres(ctx, settingsData.Database, password, settingsData.EmbeddingModel.VectorDimension)
}

// TrainProject starts chunking the project's pending documents through Docling.
// It returns immediately and reports progress through Wails events.
func (s *ProjectService) TrainProject(projectID string) (*training.Progress, error) {
	if status, ok := s.trainer.Status(projectID); ok && status.Running() {
		return status, nil
	}

	settingsData, err := s.settingsRepo.Get()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	if err := requireDatabase(settingsData.Database); err != nil {
		return nil, err
	}
	if err := requireEmbedding(settingsData.EmbeddingModel); err != nil {
		return nil, err
	}
	doclingConfig := settingsData.Docling
	if !doclingConfig.Configured {
		return nil, fmt.Errorf("document processing is not configured. Open Settings → Document processing and save it first")
	}
	if err := settings.ValidateDocling(doclingConfig); err != nil {
		return nil, fmt.Errorf("document processing is not configured correctly: %w", err)
	}

	apiKey, err := s.settingsRepo.DoclingAPIKey()
	if err != nil {
		return nil, fmt.Errorf("load Docling API key: %w", err)
	}

	chunkOptions := docling.ChunkOptions{
		MaxTokens: settingsData.EmbeddingModel.ChunkMaxTokens,
		Tokenizer: settingsData.EmbeddingModel.Tokenizer,
	}
	chunker, err := docling.NewChunker(doclingConfig, apiKey, chunkOptions)
	if err != nil {
		return nil, err
	}

	store, err := s.getStore()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := store.EnsureDocumentStatusConstraint(ctx); err != nil {
		store.Close()
		return nil, err
	}
	if err := store.EnsureChunksVectorDimension(ctx, settingsData.EmbeddingModel.VectorDimension); err != nil {
		store.Close()
		return nil, err
	}

	if _, err := store.ResetStaleProcessing(ctx, projectID); err != nil {
		store.Close()
		return nil, err
	}

	documents, err := store.ListTrainableDocuments(ctx, projectID)
	if err != nil {
		store.Close()
		return nil, err
	}

	// If all documents were already processed (e.g. user clicked "Re-index knowledge base"),
	// automatically reset them so training/reindexing can run.
	if len(documents) == 0 {
		if _, err := store.ResetDocumentsForReindex(ctx, projectID); err != nil {
			store.Close()
			return nil, err
		}
		documents, err = store.ListTrainableDocuments(ctx, projectID)
		if err != nil {
			store.Close()
			return nil, err
		}
	}

	timeout := time.Duration(doclingConfig.TimeoutSeconds) * time.Second
	pipeline, err := training.NewPipeline(store, chunker, timeout, nil)
	if err != nil {
		store.Close()
		return nil, err
	}

	progress, started := s.trainer.Start(projectID, documents, pipeline, emitTrainingProgress)
	if !started {
		// Another request won the race for this project.
		store.Close()
	}
	return progress, nil
}

// GetTrainingStatus returns the current job for a project, or nil when no job
// has been started in this app session.
func (s *ProjectService) GetTrainingStatus(projectID string) (*training.Progress, error) {
	status, ok := s.trainer.Status(projectID)
	if !ok {
		return nil, nil
	}
	return status, nil
}

func requireDatabase(cfg settings.Database) error {
	if !cfg.Configured {
		return errors.New("database is not configured. Open Settings → Database and save it first")
	}
	if err := settings.ValidateDatabase(cfg); err != nil {
		return fmt.Errorf("database settings are invalid: %w", err)
	}
	return nil
}

func requireEmbedding(cfg settings.EmbeddingModel) error {
	if !cfg.Configured {
		return errors.New("embedding model is not configured. Open Settings → Embedding Model and save it first")
	}
	if err := settings.ValidateEmbeddingModel(cfg); err != nil {
		return fmt.Errorf("embedding model settings are invalid: %w", err)
	}
	return nil
}

func emitTrainingProgress(progress training.Progress) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(trainingEventProgress, progress)
	if progress.Phase == training.PhaseFinished {
		app.Event.Emit(trainingEventFinished, progress)
	}
}
