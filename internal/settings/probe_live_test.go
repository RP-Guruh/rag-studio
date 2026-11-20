package settings

import (
	"context"
	"os"
	"strconv"
	"testing"
)

// These tests hit real services and only run when the corresponding
// environment variables are set, so `go test ./...` stays hermetic.
//
// RAG_TEST_OPENAI_BASE_URL / RAG_TEST_OPENAI_KEY / RAG_TEST_OPENAI_MODEL
// RAG_TEST_OLLAMA_BASE_URL / RAG_TEST_OLLAMA_EMBED_MODEL / RAG_TEST_OLLAMA_EMBED_DIM

func TestLiveChatModel(t *testing.T) {
	baseURL := os.Getenv("RAG_TEST_OPENAI_BASE_URL")
	apiKey := os.Getenv("RAG_TEST_OPENAI_KEY")
	model := os.Getenv("RAG_TEST_OPENAI_MODEL")
	if baseURL == "" || model == "" {
		t.Skip("set RAG_TEST_OPENAI_BASE_URL and RAG_TEST_OPENAI_MODEL to run")
	}

	result, err := TestChatModel(context.Background(), ChatModel{
		Provider:        "openai-compatible",
		BaseURL:         baseURL,
		ModelName:       model,
		Temperature:     0.2,
		MaxOutputTokens: 8,
	}, apiKey)
	if err != nil {
		t.Fatalf("live chat test: %v", err)
	}
	if !result.Success {
		t.Fatalf("live chat test failed: %s", result.Message)
	}
	t.Log(result.Message)
}

func TestLiveEmbeddingModel(t *testing.T) {
	baseURL := os.Getenv("RAG_TEST_OLLAMA_BASE_URL")
	model := os.Getenv("RAG_TEST_OLLAMA_EMBED_MODEL")
	if baseURL == "" || model == "" {
		t.Skip("set RAG_TEST_OLLAMA_BASE_URL and RAG_TEST_OLLAMA_EMBED_MODEL to run")
	}
	dimension, _ := strconv.Atoi(os.Getenv("RAG_TEST_OLLAMA_EMBED_DIM"))
	if dimension == 0 {
		dimension = 768
	}

	result, err := TestEmbeddingModel(context.Background(), EmbeddingModel{
		Provider:        "ollama",
		BaseURL:         baseURL,
		ModelName:       model,
		VectorDimension: dimension,
	}, "")
	if err != nil {
		t.Fatalf("live embedding test: %v", err)
	}
	if !result.Success {
		t.Fatalf("live embedding test failed: %s", result.Message)
	}
	t.Log(result.Message)
}
