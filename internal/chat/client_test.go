package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rag_project/internal/settings"
)

func chatConfig(provider, baseURL, model string) settings.ChatModel {
	return settings.ChatModel{
		Provider:        provider,
		BaseURL:         baseURL,
		ModelName:       model,
		Temperature:     0.2,
		MaxOutputTokens: 128,
	}
}

func TestCompleteOpenAICompatible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer key" {
			t.Errorf("Authorization = %q", got)
		}
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(request.Messages) != 2 || request.Messages[0].Role != "system" ||
			request.Messages[1].Content != "question" {
			t.Errorf("unexpected messages: %+v", request.Messages)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"  answer  "}}]}`))
	}))
	defer server.Close()

	client, err := New(chatConfig("openai-compatible", server.URL+"/v1", "gpt-4o"), "key")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	answer, err := client.Complete(context.Background(), "system", "question")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if answer != "answer" {
		t.Fatalf("answer = %q", answer)
	}
}

func TestCompleteAnthropic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "key" || r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing Anthropic headers")
		}
		w.Write([]byte(`{"content":[{"type":"text","text":"claude answer"}]}`))
	}))
	defer server.Close()

	client, err := New(chatConfig("anthropic", server.URL+"/v1", "claude"), "key")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	answer, err := client.Complete(context.Background(), "system", "question")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if answer != "claude answer" {
		t.Fatalf("answer = %q", answer)
	}
}

func TestCompleteOllama(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Write([]byte(`{"message":{"content":"local answer"}}`))
	}))
	defer server.Close()

	client, err := New(chatConfig("ollama", server.URL, "llama3"), "")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	answer, err := client.Complete(context.Background(), "system", "question")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if answer != "local answer" {
		t.Fatalf("answer = %q", answer)
	}
}

func TestCompleteHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"backend down"}`))
	}))
	defer server.Close()

	client, err := New(chatConfig("openai-compatible", server.URL, "model"), "")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	_, err = client.Complete(context.Background(), "system", "question")
	if err == nil || !strings.Contains(err.Error(), "backend down") {
		t.Fatalf("expected backend error, got %v", err)
	}
}
