package service

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"rag_project/internal/settings"
)

type testSecrets struct {
	mu     sync.Mutex
	values map[string]string
}

func (s *testSecrets) Get(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values[name], nil
}

func (s *testSecrets) Set(name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name] = value
	return nil
}

func (s *testSecrets) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, name)
	return nil
}

func newTestService(t *testing.T) *SettingsService {
	t.Helper()
	store, err := settings.Open(filepath.Join(t.TempDir(), "settings.db"), &testSecrets{values: map[string]string{}})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return NewSettingsService(store)
}

func TestSettingsServiceRoundTrip(t *testing.T) {
	service := newTestService(t)

	saved, err := service.SaveChatModel(settings.ChatModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "model-x",
		Temperature:     0.4,
		MaxOutputTokens: 2048,
		APIKey:          "sk-test-5678",
	})
	if err != nil {
		t.Fatalf("save chat model: %v", err)
	}
	if !saved.APIKeySet || saved.APIKeyHint != "…5678" {
		t.Fatalf("unexpected secret state: %+v", saved)
	}

	all, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if all.ChatModel.ModelName != "model-x" {
		t.Errorf("model = %q", all.ChatModel.ModelName)
	}
	if all.ChatModel.APIKey != "" {
		t.Error("GetSettings must not return the API key")
	}
	if all.EmbeddingModel.Configured || all.EmbeddingModel.VectorDimension != 0 {
		t.Errorf("embedding model must stay unconfigured without explicit save: %+v", all.EmbeddingModel)
	}

	if _, err := service.SaveChatModel(settings.ChatModel{
		Provider: "openai-compatible", BaseURL: "https://api.example.com/v1",
		ModelName: "model-y", Temperature: 5, MaxOutputTokens: 100,
	}); err == nil {
		t.Error("expected validation error for out-of-range temperature")
	}
}

// TestSettingsServiceDatabaseUsesStoredPassword verifies that a connection test
// can reuse a saved password and reaches a real pgvector instance.
func TestSettingsServiceDatabaseUsesStoredPassword(t *testing.T) {
	password := getenv("RAG_TEST_PG_PASSWORD")
	if password == "" {
		t.Skip("set RAG_TEST_PG_PASSWORD to run")
	}
	service := newTestService(t)

	if _, err := service.SaveEmbeddingModel(settings.EmbeddingModel{
		Provider:        "ollama",
		BaseURL:         "http://localhost:11434",
		ModelName:       "bge-m3",
		VectorDimension: 1024,
		ChunkMaxTokens:  512,
		Tokenizer:       "BAAI/bge-m3",
	}); err != nil {
		t.Fatalf("save embedding model: %v", err)
	}

	port := 5432
	if getenv("RAG_TEST_PG_PORT") == "8432" {
		port = 8432
	}
	saved, err, _ := service.SaveDatabase(settings.Database{
		Host:         getenvOr("RAG_TEST_PG_HOST", "127.0.0.1"),
		Port:         port,
		DatabaseName: getenvOr("RAG_TEST_PG_DATABASE", "postgres"),
		Username:     getenvOr("RAG_TEST_PG_USER", "postgres"),
		Password:     password,
		SSLMode:      "disable",
		SchemaName:   "public",
	})
	if err != nil {
		t.Fatalf("save database: %v", err)
	}
	if !saved.PasswordSet {
		t.Fatal("password should be marked as stored")
	}

	result, err := service.TestDatabase(saved)
	if err != nil {
		t.Fatalf("test database: %v", err)
	}
	if !result.Success {
		t.Fatalf("database test failed: %s", result.Message)
	}
	t.Log(result.Message)
}

func TestSettingsServiceDoclingRoundTrip(t *testing.T) {
	service := newTestService(t)

	saved, err := service.SaveDocling(settings.DoclingConnection{
		Mode:           "serve",
		BaseURL:        "http://docling.local:5001",
		TimeoutSeconds: 600,
		APIKey:         "docling-key-9876",
	})
	if err != nil {
		t.Fatalf("save docling: %v", err)
	}
	if !saved.APIKeySet || saved.APIKeyHint != "…9876" {
		t.Fatalf("unexpected secret state: %+v", saved)
	}

	all, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if all.Docling.BaseURL != "http://docling.local:5001" || all.Docling.TimeoutSeconds != 600 {
		t.Errorf("docling = %+v", all.Docling)
	}
	if all.Docling.APIKey != "" {
		t.Error("GetSettings must not return the Docling API key")
	}

	if _, err := service.SaveDocling(settings.DoclingConnection{
		Mode: "local", TimeoutSeconds: 1,
	}); err == nil {
		t.Error("expected validation error for out-of-range timeout")
	}
}

func getenv(key string) string {
	return os.Getenv(key)
}

func getenvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
