package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rag_project/internal/settings"
)

func testConfig(provider, baseURL, model string, dimension int) settings.EmbeddingModel {
	return settings.EmbeddingModel{
		Provider:        provider,
		BaseURL:         baseURL,
		ModelName:       model,
		VectorDimension: dimension,
		ChunkMaxTokens:  512,
		Tokenizer:       "BAAI/bge-m3",
	}
}

func TestEmbedOpenAICompatibleSortsByIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("path = %s, want /v1/embeddings", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		var request struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "text-embedding-3-small" || strings.Join(request.Input, "|") != "first|second" {
			t.Errorf("unexpected request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"index":1,"embedding":[0.3,0.4,0.5]},{"index":0,"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer server.Close()

	embedder, err := New(testConfig("openai-compatible", server.URL+"/v1", "text-embedding-3-small", 3), "secret")
	if err != nil {
		t.Fatalf("new embedder: %v", err)
	}
	vectors, err := embedder.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vectors) != 2 || vectors[0][0] != 0.1 || vectors[1][0] != 0.3 {
		t.Fatalf("vectors out of order: %+v", vectors)
	}
	if embedder.Dimension() != 3 || embedder.Model() != "text-embedding-3-small" {
		t.Errorf("unexpected metadata: dim=%d model=%s", embedder.Dimension(), embedder.Model())
	}
}

func TestEmbedRejectsWrongDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2]}]}`))
	}))
	defer server.Close()

	embedder, err := New(testConfig("openai-compatible", server.URL, "model", 3), "")
	if err != nil {
		t.Fatalf("new embedder: %v", err)
	}
	if _, err := embedder.Embed(context.Background(), []string{"hello"}); err == nil ||
		!strings.Contains(err.Error(), "expected 3") {
		t.Fatalf("expected dimension error, got %v", err)
	}
}

func TestEmbedOllama(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path = %s, want /api/embed", r.URL.Path)
		}
		w.Write([]byte(`{"embeddings":[[1,2,3]]}`))
	}))
	defer server.Close()

	embedder, err := New(testConfig("ollama", server.URL, "bge-m3", 3), "")
	if err != nil {
		t.Fatalf("new embedder: %v", err)
	}
	vectors, err := embedder.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vectors) != 1 || len(vectors[0]) != 3 {
		t.Fatalf("unexpected vectors: %+v", vectors)
	}
}

func TestEmbedHuggingFaceMeanPoolsTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/sentence-transformers/all-MiniLM-L6-v2" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`[[[1,3],[3,5]]]`))
	}))
	defer server.Close()

	embedder, err := New(testConfig("huggingface", server.URL, "sentence-transformers/all-MiniLM-L6-v2", 2), "")
	if err != nil {
		t.Fatalf("new embedder: %v", err)
	}
	vectors, err := embedder.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vectors) != 1 || vectors[0][0] != 2 || vectors[0][1] != 4 {
		t.Fatalf("mean pool = %+v, want [[2 4]]", vectors)
	}
}

func TestEmbedBatchesRequests(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Write([]byte(`{"data":[{"index":0,"embedding":[0,0]},{"index":1,"embedding":[1,1]}]}`))
	}))
	defer server.Close()

	embedder, err := New(testConfig("openai-compatible", server.URL, "model", 2), "")
	if err != nil {
		t.Fatalf("new embedder: %v", err)
	}
	embedder.(*client).batchSize = 2
	vectors, err := embedder.Embed(context.Background(), []string{"a", "b", "c", "d"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if requests != 2 || len(vectors) != 4 {
		t.Fatalf("requests = %d, vectors = %d", requests, len(vectors))
	}
}
