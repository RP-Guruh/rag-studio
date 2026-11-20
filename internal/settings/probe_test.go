package settings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestChatModelOpenAICompatible(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["model"] != "gpt-test" {
			t.Errorf("model = %v", body["model"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	result, err := TestChatModel(context.Background(), ChatModel{
		Provider:        "openai-compatible",
		BaseURL:         server.URL + "/v1",
		ModelName:       "gpt-test",
		Temperature:     0.2,
		MaxOutputTokens: 16,
	}, "sk-test")
	if err != nil {
		t.Fatalf("test chat: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Message)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("authorization header = %q", gotAuth)
	}
}

func TestChatModelReportsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key","type":"auth_error"}}`))
	}))
	defer server.Close()

	result, err := TestChatModel(context.Background(), ChatModel{
		Provider:        "openai-compatible",
		BaseURL:         server.URL + "/v1",
		ModelName:       "gpt-test",
		MaxOutputTokens: 16,
	}, "bad")
	if err != nil {
		t.Fatalf("test chat: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure")
	}
	if !strings.Contains(result.Message, "invalid api key") {
		t.Errorf("message = %q", result.Message)
	}
	if strings.Contains(result.Message, "bad") {
		t.Errorf("message must not leak the API key: %q", result.Message)
	}
}

func TestChatModelAnthropic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "anthropic-key" {
			t.Errorf("missing x-api-key")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing anthropic-version")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer server.Close()

	result, err := TestChatModel(context.Background(), ChatModel{
		Provider:        "anthropic",
		BaseURL:         server.URL + "/v1",
		ModelName:       "claude-test",
		MaxOutputTokens: 16,
	}, "anthropic-key")
	if err != nil {
		t.Fatalf("test chat: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Message)
	}
}

func TestChatModelOllama(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{"message":{"role":"assistant","content":"ok"},"done":true}`))
	}))
	defer server.Close()

	result, err := TestChatModel(context.Background(), ChatModel{
		Provider:        "ollama",
		BaseURL:         server.URL,
		ModelName:       "llama3",
		MaxOutputTokens: 16,
	}, "")
	if err != nil {
		t.Fatalf("test chat: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Message)
	}
}

func TestEmbeddingDimensionMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer server.Close()

	result, err := TestEmbeddingModel(context.Background(), EmbeddingModel{
		Provider:        "openai-compatible",
		BaseURL:         server.URL + "/v1",
		ModelName:       "embed-test",
		VectorDimension: 3,
	}, "sk-test")
	if err != nil {
		t.Fatalf("test embedding: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "3 dimensions") {
		t.Errorf("message = %q", result.Message)
	}
}

func TestEmbeddingDimensionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer server.Close()

	result, err := TestEmbeddingModel(context.Background(), EmbeddingModel{
		Provider:        "openai-compatible",
		BaseURL:         server.URL + "/v1",
		ModelName:       "embed-test",
		VectorDimension: 1536,
	}, "")
	if err != nil {
		t.Fatalf("test embedding: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for dimension mismatch")
	}
	if !strings.Contains(result.Message, "1536") {
		t.Errorf("message = %q", result.Message)
	}
}

func TestEmbeddingOllamaAndHuggingFace(t *testing.T) {
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("unexpected ollama path %q", r.URL.Path)
		}
		w.Write([]byte(`{"embeddings":[[0.1,0.2]]}`))
	}))
	defer ollama.Close()

	result, err := TestEmbeddingModel(context.Background(), EmbeddingModel{
		Provider: "ollama", BaseURL: ollama.URL, ModelName: "nomic", VectorDimension: 2,
	}, "")
	if err != nil || !result.Success {
		t.Fatalf("ollama embedding: err=%v result=%+v", err, result)
	}

	hf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/sentence-transformers/all-MiniLM-L6-v2" {
			t.Errorf("unexpected huggingface path %q", r.URL.Path)
		}
		w.Write([]byte(`[0.1,0.2,0.3,0.4]`))
	}))
	defer hf.Close()

	result, err = TestEmbeddingModel(context.Background(), EmbeddingModel{
		Provider:        "huggingface",
		BaseURL:         hf.URL,
		ModelName:       "sentence-transformers/all-MiniLM-L6-v2",
		VectorDimension: 4,
	}, "")
	if err != nil || !result.Success {
		t.Fatalf("huggingface embedding: err=%v result=%+v", err, result)
	}
}

func TestUnreachableEndpointFailsGracefully(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	result, err := TestChatModel(context.Background(), ChatModel{
		Provider: "openai-compatible", BaseURL: url + "/v1", ModelName: "m", MaxOutputTokens: 1,
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for a closed server")
	}
	if !strings.Contains(result.Message, "Could not reach") {
		t.Errorf("message = %q", result.Message)
	}
}

func TestPostgresDSNEscapesCredentials(t *testing.T) {
	dsn, err := postgresDSN(Database{
		Host: "localhost", Port: 5432, DatabaseName: "rag",
		Username: "user name", SSLMode: "verify-full",
	}, "p@ss:/word?")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	if !strings.Contains(dsn, "user%20name") {
		t.Errorf("username not escaped: %q", dsn)
	}
	if strings.Contains(dsn, "p@ss:/word?") {
		t.Errorf("password not escaped: %q", dsn)
	}
	if !strings.Contains(dsn, "sslmode=verify-full") {
		t.Errorf("ssl mode missing: %q", dsn)
	}
}

func TestDatabaseRejectsInvalidSettings(t *testing.T) {
	_, err := TestDatabase(context.Background(), Database{
		Host: "", Port: 5432, DatabaseName: "rag", Username: "u", SSLMode: "disable",
	}, "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDatabaseUnreachablePort(t *testing.T) {
	result, err := TestDatabase(context.Background(), Database{
		Host: "127.0.0.1", Port: 1, DatabaseName: "postgres",
		Username: "postgres", SSLMode: "disable", SchemaName: "public",
	}, "nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected connection failure")
	}
	if !strings.Contains(result.Message, "Could not connect") {
		t.Errorf("message = %q", result.Message)
	}
}

// TestDatabaseLivePostgres runs only when RAG_TEST_PG_PASSWORD is set, so the
// default test suite stays hermetic.
func TestDatabaseLivePostgres(t *testing.T) {
	password := os.Getenv("RAG_TEST_PG_PASSWORD")
	if password == "" {
		t.Skip("set RAG_TEST_PG_PASSWORD to run the live PostgreSQL test")
	}
	host := envOr("RAG_TEST_PG_HOST", "127.0.0.1")
	port := 0
	switch envOr("RAG_TEST_PG_PORT", "8432") {
	case "5432":
		port = 5432
	case "8432":
		port = 8432
	default:
		t.Fatalf("unsupported test port")
	}
	dbName := envOr("RAG_TEST_PG_DATABASE", "postgres")
	user := envOr("RAG_TEST_PG_USER", "postgres")

	result, err := TestDatabase(context.Background(), Database{
		Host: host, Port: port, DatabaseName: dbName, Username: user,
		SSLMode: "disable", SchemaName: "public",
	}, password)
	if err != nil {
		t.Fatalf("live database test: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected live connection to succeed, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "pgvector") {
		t.Errorf("message = %q", result.Message)
	}
}

func TestDoclingServeHealth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"2.1.0"}`))
	}))
	defer server.Close()

	result, err := TestDocling(context.Background(), DoclingConnection{
		Mode:           "serve",
		BaseURL:        server.URL,
		TimeoutSeconds: 300,
	}, "dl-key")
	if err != nil {
		t.Fatalf("test docling: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "2.1.0") {
		t.Errorf("message = %q", result.Message)
	}
	if gotAuth != "Bearer dl-key" {
		t.Errorf("authorization header = %q", gotAuth)
	}
}

func TestDoclingServeReportsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":"invalid api key"}`))
	}))
	defer server.Close()

	result, err := TestDocling(context.Background(), DoclingConnection{
		Mode:           "serve",
		BaseURL:        server.URL,
		TimeoutSeconds: 300,
	}, "")
	if err != nil {
		t.Fatalf("test docling: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for HTTP 401")
	}
	if !strings.Contains(result.Message, "401") {
		t.Errorf("message = %q", result.Message)
	}
}

func TestDoclingLocalModeUsesHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	result, err := TestDocling(context.Background(), DoclingConnection{
		Mode:           "local",
		BaseURL:        server.URL,
		TimeoutSeconds: 300,
	}, "")
	if err != nil {
		t.Fatalf("test docling: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "Local Docling") {
		t.Errorf("message = %q", result.Message)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
