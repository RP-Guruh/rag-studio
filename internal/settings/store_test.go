package settings

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type memorySecrets struct {
	mu     sync.Mutex
	values map[string]string
}

func newMemorySecrets() *memorySecrets {
	return &memorySecrets{values: map[string]string{}}
}

func (m *memorySecrets) Get(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.values[name], nil
}

func (m *memorySecrets) Set(name, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[name] = value
	return nil
}

func (m *memorySecrets) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, name)
	return nil
}

func newTestStore(t *testing.T) (*Store, *memorySecrets) {
	t.Helper()
	secrets := newMemorySecrets()
	store, err := Open(filepath.Join(t.TempDir(), "settings.db"), secrets)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store, secrets
}

func TestGetReturnsUnconfiguredWhenEmpty(t *testing.T) {
	store, _ := newTestStore(t)

	got, err := store.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ChatModel.Configured || got.EmbeddingModel.Configured ||
		got.Database.Configured || got.Docling.Configured {
		t.Errorf("sections must not be marked configured: %+v", got)
	}
	if got.ChatModel.Provider != "" || got.ChatModel.ModelName != "" {
		t.Errorf("chat model must be empty, got %+v", got.ChatModel)
	}
	if got.EmbeddingModel.VectorDimension != 0 || got.EmbeddingModel.ChunkMaxTokens != 0 ||
		got.EmbeddingModel.Tokenizer != "" {
		t.Errorf("embedding model must be empty, got %+v", got.EmbeddingModel)
	}
	if got.Database.Host != "" || got.Database.Port != 0 || got.Database.SSLMode != "" {
		t.Errorf("database must be empty, got %+v", got.Database)
	}
	if got.Docling.Mode != "" || got.Docling.BaseURL != "" || got.Docling.TimeoutSeconds != 0 {
		t.Errorf("docling must be empty, got %+v", got.Docling)
	}
}

func TestSaveChatModelKeepsSingleRowAndManagesSecret(t *testing.T) {
	store, secrets := newTestStore(t)

	saved, err := store.SaveChatModel(ChatModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "test-model",
		Temperature:     0.7,
		MaxOutputTokens: 512,
		APIKey:          "sk-secret-abcd1234",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if !saved.APIKeySet {
		t.Error("APIKeySet should be true after saving a key")
	}
	if saved.APIKeyHint != "…1234" {
		t.Errorf("hint = %q, want …1234", saved.APIKeyHint)
	}
	if saved.APIKey != "" {
		t.Error("saved settings must not echo the API key")
	}
	if saved.UpdatedAt == "" {
		t.Error("updatedAt should be populated")
	}
	if got, _ := secrets.Get(secretChatModel); got != "sk-secret-abcd1234" {
		t.Errorf("stored secret = %q", got)
	}

	assertRowCount(t, store, "chat_model_settings", 1)

	// Saving without a key must keep the existing secret.
	second, err := store.SaveChatModel(ChatModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "test-model-2",
		Temperature:     0.1,
		MaxOutputTokens: 256,
	})
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if !second.APIKeySet || second.APIKeyHint != "…1234" {
		t.Errorf("secret state lost: %+v", second)
	}
	if got, _ := secrets.Get(secretChatModel); got != "sk-secret-abcd1234" {
		t.Errorf("secret was overwritten: %q", got)
	}
	if second.ModelName != "test-model-2" {
		t.Errorf("model = %q", second.ModelName)
	}
	assertRowCount(t, store, "chat_model_settings", 1)

	// A new key replaces the old one.
	if _, err := store.SaveChatModel(ChatModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "test-model-2",
		Temperature:     0.1,
		MaxOutputTokens: 256,
		APIKey:          "sk-rotated-9999",
	}); err != nil {
		t.Fatalf("rotate key: %v", err)
	}
	if got, _ := secrets.Get(secretChatModel); got != "sk-rotated-9999" {
		t.Errorf("secret = %q after rotation", got)
	}

	// Clearing removes the secret but keeps the row.
	cleared, err := store.SaveChatModel(ChatModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "test-model-2",
		Temperature:     0.1,
		MaxOutputTokens: 256,
		ClearAPIKey:     true,
	})
	if err != nil {
		t.Fatalf("clear key: %v", err)
	}
	if cleared.APIKeySet || cleared.APIKeyHint != "" {
		t.Errorf("secret state after clear = %+v", cleared)
	}
	if got, _ := secrets.Get(secretChatModel); got != "" {
		t.Errorf("secret = %q after clear", got)
	}
	assertRowCount(t, store, "chat_model_settings", 1)
}

func TestSaveEmbeddingModel(t *testing.T) {
	store, _ := newTestStore(t)

	if _, err := store.SaveEmbeddingModel(EmbeddingModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "missing-chunk-options",
		VectorDimension: 1024,
	}); err == nil {
		t.Error("expected validation error when chunk options are missing")
	}

	saved, err := store.SaveEmbeddingModel(EmbeddingModel{
		Provider:        "Ollama",
		BaseURL:         "http://localhost:11434/",
		ModelName:       "nomic-embed-text",
		VectorDimension: 768,
		ChunkMaxTokens:  384,
		Tokenizer:       "nomic-ai/nomic-embed-text-v1.5",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Provider != "ollama" {
		t.Errorf("provider = %q, want ollama", saved.Provider)
	}
	if saved.VectorDimension != 768 {
		t.Errorf("dimension = %d", saved.VectorDimension)
	}
	if saved.ChunkMaxTokens != 384 || saved.Tokenizer != "nomic-ai/nomic-embed-text-v1.5" {
		t.Errorf("chunk options = %+v", saved)
	}
	if !saved.Configured {
		t.Error("saved embedding model must be marked configured")
	}
	assertRowCount(t, store, "embedding_model_settings", 1)

	updated, err := store.SaveEmbeddingModel(EmbeddingModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "custom-embed",
		VectorDimension: 1024,
		ChunkMaxTokens:  256,
		Tokenizer:       "BAAI/bge-large-en-v1.5",
	})
	if err != nil {
		t.Fatalf("save custom chunk options: %v", err)
	}
	if updated.ChunkMaxTokens != 256 || updated.Tokenizer != "BAAI/bge-large-en-v1.5" {
		t.Errorf("chunk options not persisted: %+v", updated)
	}

	if _, err := store.SaveEmbeddingModel(EmbeddingModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "x",
		VectorDimension: 0,
	}); err == nil {
		t.Error("expected validation error for zero dimension")
	}

	if _, err := store.SaveEmbeddingModel(EmbeddingModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "x",
		VectorDimension: 1536,
		ChunkMaxTokens:  9000,
		Tokenizer:       "BAAI/bge-m3",
	}); err == nil {
		t.Error("expected validation error for out-of-range chunk max tokens")
	}
}

func TestSaveDatabase(t *testing.T) {
	store, secrets := newTestStore(t)

	// First save - should succeed without migration (no prior settings)
	saved, saveErr, _ := store.SaveDatabase(Database{
		Host:         "db.internal",
		Port:         6543,
		DatabaseName: "rag",
		Username:     "app",
		Password:     "super-secret-password",
		SSLMode:      "require",
	})

	if saveErr != nil {
		t.Fatalf("save: %v", saveErr)
	}
	if !saved.PasswordSet || saved.PasswordHint != "…word" && saved.PasswordHint != "••••" {
		t.Errorf("password state = %+v", saved)
	}
	if saved.SchemaName != "public" {
		t.Errorf("schema = %q, want public", saved.SchemaName)
	}
	if got, _ := secrets.Get(secretDatabase); got != "super-secret-password" {
		t.Errorf("stored password = %q", got)
	}
	assertRowCount(t, store, "database_settings", 1)

	// Second save with invalid port - should fail validation
	_, errPort, _ := store.SaveDatabase(Database{
		Host: "h", Port: 70000, DatabaseName: "d", Username: "u", SSLMode: "disable",
	})
	if errPort == nil {
		t.Error("expected validation error for invalid port")
	}

	// Third save with invalid ssl mode - should fail validation
	_, errSSL, _ := store.SaveDatabase(Database{
		Host: "h", Port: 5432, DatabaseName: "d", Username: "u", SSLMode: "bogus",
	})
	if errSSL == nil {
		t.Error("expected validation error for invalid ssl mode")
	}
}
func TestValidationRejectsOutOfRangeTemperature(t *testing.T) {
	base := ChatModel{
		Provider:        "openai-compatible",
		BaseURL:         "https://api.example.com/v1",
		ModelName:       "m",
		Temperature:     2.5,
		MaxOutputTokens: 100,
	}
	if err := ValidateChatModel(base); err == nil {
		t.Errorf("expected temperature validation error for value %v", base.Temperature)
	}
}
func TestSchemaRejectsSecondRow(t *testing.T) {
	store, _ := newTestStore(t)

	_, err := store.db.Exec(`
		INSERT INTO chat_model_settings (id, base_url, model_name)
		VALUES (2, 'https://example.com', 'model')`)
	if err == nil {
		t.Fatal("expected CHECK constraint to reject id = 2")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "constraint") {
		t.Logf("unexpected error text: %v", err)
	}
	assertRowCount(t, store, "chat_model_settings", 0)
}

func TestReopenKeepsData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.db")
	secrets := newMemorySecrets()

	store, err := Open(path, secrets)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := store.SaveChatModel(ChatModel{
		Provider: "ollama", BaseURL: "http://localhost:11434", ModelName: "llama3",
		Temperature: 0.2, MaxOutputTokens: 1024,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	store.Close()

	reopened, err := Open(path, secrets)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	got, err := reopened.GetChatModel()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ModelName != "llama3" {
		t.Errorf("model = %q after reopen", got.ModelName)
	}
}

func TestSaveDoclingKeepsSingleRowAndManagesSecret(t *testing.T) {
	store, secrets := newTestStore(t)

	saved, err := store.SaveDocling(DoclingConnection{
		Mode:           "SERVE",
		BaseURL:        "  https://docling.example.com  ",
		TimeoutSeconds: 600,
		APIKey:         "dl-secret-1234",
	})
	if err != nil {
		t.Fatalf("save docling: %v", err)
	}
	if saved.Mode != "serve" {
		t.Errorf("mode = %q, want serve", saved.Mode)
	}
	if saved.BaseURL != "https://docling.example.com" {
		t.Errorf("base url = %q", saved.BaseURL)
	}
	if !saved.APIKeySet || saved.APIKeyHint != "…1234" {
		t.Errorf("unexpected secret state: %+v", saved)
	}
	if got := secrets.values[secretDocling]; got != "dl-secret-1234" {
		t.Errorf("stored secret = %q", got)
	}
	if saved.APIKey != "" {
		t.Error("saved result must not expose the API key")
	}
	assertRowCount(t, store, "docling_connection", 1)

	// A second save must update the same row, not add another.
	if _, err := store.SaveDocling(DoclingConnection{
		Mode: "local", BaseURL: "http://localhost:5001", TimeoutSeconds: 120,
	}); err != nil {
		t.Fatalf("second save: %v", err)
	}
	assertRowCount(t, store, "docling_connection", 1)

	reloaded, err := store.GetDocling()
	if err != nil {
		t.Fatalf("get docling: %v", err)
	}
	if !reloaded.APIKeySet || reloaded.APIKeyHint != "…1234" {
		// An empty APIKey keeps the stored secret, so it should still be set.
		t.Errorf("empty API key should keep the stored secret: %+v", reloaded)
	}

	if _, err := store.SaveDocling(DoclingConnection{
		Mode: "local", BaseURL: "http://localhost:5001", TimeoutSeconds: 120, ClearAPIKey: true,
	}); err != nil {
		t.Fatalf("clear secret: %v", err)
	}
	if got := secrets.values[secretDocling]; got != "" {
		t.Errorf("secret should be removed, got %q", got)
	}
}

func TestDoclingTableRejectsSecondRow(t *testing.T) {
	store, _ := newTestStore(t)

	if _, err := store.db.Exec(`
		INSERT INTO docling_connection (id, mode, base_url, timeout_seconds)
		VALUES (2, 'serve', 'http://localhost:5001', 300)`); err == nil {
		t.Fatal("expected the single-row constraint to reject id = 2")
	}
	assertRowCount(t, store, "docling_connection", 0)
}

func TestValidateDocling(t *testing.T) {
	if err := ValidateDocling(DoclingConnection{Mode: "bogus", TimeoutSeconds: 300}); err == nil {
		t.Error("expected unsupported mode error")
	}
	if err := ValidateDocling(DoclingConnection{Mode: "serve", BaseURL: "", TimeoutSeconds: 300}); err == nil {
		t.Error("expected base URL error")
	}
	if err := ValidateDocling(DoclingConnection{Mode: "local", BaseURL: "", TimeoutSeconds: 300}); err == nil {
		t.Error("expected base URL error in local mode too")
	}
	if err := ValidateDocling(DoclingConnection{Mode: "local", BaseURL: "http://localhost:5001", TimeoutSeconds: 300}); err != nil {
		t.Errorf("local mode with a base URL should be valid: %v", err)
	}
	if err := ValidateDocling(DoclingConnection{Mode: "local", BaseURL: "http://localhost:5001", TimeoutSeconds: 0}); err == nil {
		t.Error("expected timeout error")
	}
}

func TestSecretHint(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"abc":      "••••",
		"abcd":     "••••",
		"abcdefgh": "…efgh",
	}
	for input, want := range cases {
		if got := SecretHint(input); got != want {
			t.Errorf("SecretHint(%q) = %q, want %q", input, got, want)
		}
	}
}

func assertRowCount(t *testing.T, store *Store, table string, want int) {
	t.Helper()
	var got int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Errorf("%s row count = %d, want %d", table, got, want)
	}
}
