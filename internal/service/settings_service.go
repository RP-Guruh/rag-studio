package service

import (
	"context"
	"fmt"
	"time"

	"rag_project/internal/projects"
	"rag_project/internal/settings"
)

// SettingsService exposes the settings store to the Vue frontend through Wails
// bindings.
type SettingsService struct {
	repository settings.Repository
}

// NewSettingsService wires a repository into a bindable service.
func NewSettingsService(repository settings.Repository) *SettingsService {
	return &SettingsService{repository: repository}
}

// GetSettings returns every configuration section. Secrets are never
// included; only their set flag and hint.
func (s *SettingsService) GetSettings() (settings.Settings, error) {
	return s.repository.Get()
}

// SaveChatModel validates and persists the chat model configuration.
func (s *SettingsService) SaveChatModel(input settings.ChatModel) (settings.ChatModel, error) {
	return s.repository.SaveChatModel(input)
}

// SaveEmbeddingModel validates and persists the embedding model configuration.
func (s *SettingsService) SaveEmbeddingModel(input settings.EmbeddingModel) (settings.EmbeddingModel, error) {
	return s.repository.SaveEmbeddingModel(input)
}

// SaveDatabase validates and persists the PostgreSQL configuration.
// If migrate is true, it also executes PostgreSQL table migrations.
func (s *SettingsService) SaveDatabase(input settings.Database) (settings.Database, error, bool) {
	saved, err, _ := s.repository.SaveDatabase(input)
	if err != nil {
		return settings.Database{}, err, false
	}

	// Always ensure PostgreSQL tables are created/migrated when saving database settings
	password := input.Password
	if password == "" {
		stored, err := s.repository.DatabasePassword()
		if err == nil {
			password = stored
		}
	}
	embedding, err := s.repository.GetEmbeddingModel()
	if err != nil {
		return saved, fmt.Errorf("load embedding model settings: %w", err), false
	}
	if !embedding.Configured || embedding.VectorDimension <= 0 {
		return saved, fmt.Errorf(
			"database settings saved, but the PostgreSQL schema was not migrated: configure the embedding model and vector dimension first",
		), false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Attempt PostgreSQL migration with the configured vector dimension.
	if migErr := projects.MigratePostgres(ctx, saved, password, embedding.VectorDimension); migErr != nil {
		return saved, fmt.Errorf("settings saved in SQLite, but PostgreSQL table migration failed: %w", migErr), false
	}

	return saved, nil, true
}

// SaveDocling validates and persists the Docling document-processing
// connection.
func (s *SettingsService) SaveDocling(input settings.DoclingConnection) (settings.DoclingConnection, error) {
	return s.repository.SaveDocling(input)
}

// TestChatModel performs a live round-trip. An empty API key reuses the stored
// one, which lets the user test without retyping the secret.
func (s *SettingsService) TestChatModel(input settings.ChatModel) (settings.TestResult, error) {
	apiKey := input.APIKey
	if apiKey == "" {
		stored, err := s.repository.ChatModelAPIKey()
		if err != nil {
			return settings.TestResult{Success: false, Message: err.Error()}, nil
		}
		apiKey = stored
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return settings.TestChatModel(ctx, input, apiKey)
}

// TestEmbeddingModel performs a live embedding call and verifies the vector
// dimension.
func (s *SettingsService) TestEmbeddingModel(input settings.EmbeddingModel) (settings.TestResult, error) {
	apiKey := input.APIKey
	if apiKey == "" {
		stored, err := s.repository.EmbeddingAPIKey()
		if err != nil {
			return settings.TestResult{Success: false, Message: err.Error()}, nil
		}
		apiKey = stored
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return settings.TestEmbeddingModel(ctx, input, apiKey)
}

// TestDatabase connects to PostgreSQL and checks for the pgvector extension.
func (s *SettingsService) TestDatabase(input settings.Database) (settings.TestResult, error) {
	password := input.Password
	if password == "" {
		stored, err := s.repository.DatabasePassword()
		if err != nil {
			return settings.TestResult{Success: false, Message: err.Error()}, nil
		}
		password = stored
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return settings.TestDatabase(ctx, input, password)
}

// TestDocling reaches the Docling engine. In serve mode an empty API key reuses
// the stored one; in local mode it checks the installed CLI.
func (s *SettingsService) TestDocling(input settings.DoclingConnection) (settings.TestResult, error) {
	apiKey := input.APIKey
	if apiKey == "" {
		stored, err := s.repository.DoclingAPIKey()
		if err != nil {
			return settings.TestResult{Success: false, Message: err.Error()}, nil
		}
		apiKey = stored
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return settings.TestDocling(ctx, input, apiKey)
}

// ChatModelAPIKey returns the stored chat API key, or an empty string.
func (s *SettingsService) ChatModelAPIKey() (string, error) {
	return s.repository.ChatModelAPIKey()
}

// EmbeddingAPIKey returns the stored embedding API key, or an empty string.
func (s *SettingsService) EmbeddingAPIKey() (string, error) {
	return s.repository.EmbeddingAPIKey()
}

// DatabasePassword returns the stored database password, or an empty string.
func (s *SettingsService) DatabasePassword() (string, error) {
	return s.repository.DatabasePassword()
}

// DoclingAPIKey returns the stored Docling API key, or an empty string.
func (s *SettingsService) DoclingAPIKey() (string, error) {
	return s.repository.DoclingAPIKey()
}
