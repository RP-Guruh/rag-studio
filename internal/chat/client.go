// Package chat calls the configured chat model to synthesize an answer from a
// system prompt (the project persona) and a user prompt (question + context).
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"rag_project/internal/settings"
)

const (
	maxResponseBytes   = 1 << 20
	defaultHTTPTimeout = 120 * time.Second
)

// Client completes a single chat turn, with or without streaming.
type Client interface {
	Complete(ctx context.Context, system, user string) (string, error)
	Stream(ctx context.Context, system, user string, onToken func(string)) (string, error)
}

type client struct {
	provider    string
	baseURL     string
	model       string
	apiKey      string
	temperature float64
	maxTokens   int
	http        *http.Client
}

// New builds a Client from the saved chat model settings.
func New(cfg settings.ChatModel, apiKey string) (Client, error) {
	if err := settings.ValidateChatModel(cfg); err != nil {
		return nil, err
	}
	return &client{
		provider:    settings.NormalizeProvider(cfg.Provider),
		baseURL:     strings.TrimSpace(cfg.BaseURL),
		model:       strings.TrimSpace(cfg.ModelName),
		apiKey:      strings.TrimSpace(apiKey),
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxOutputTokens,
		http:        &http.Client{Timeout: defaultHTTPTimeout},
	}, nil
}

func (c *client) Complete(ctx context.Context, system, user string) (string, error) {
	switch c.provider {
	case "openai-compatible":
		return c.completeOpenAI(ctx, system, user)
	case "anthropic":
		return c.completeAnthropic(ctx, system, user)
	case "ollama":
		return c.completeOllama(ctx, system, user)
	default:
		return "", fmt.Errorf("unsupported chat provider %q", c.provider)
	}
}

func (c *client) completeOpenAI(ctx context.Context, system, user string) (string, error) {
	headers := map[string]string{}
	if c.apiKey != "" {
		headers["Authorization"] = "Bearer " + c.apiKey
	}
	body, err := c.post(ctx, joinURL(c.baseURL, "chat/completions"), headers, map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": c.temperature,
		"max_tokens":  c.maxTokens,
		"stream":      false,
	})
	if err != nil {
		return "", err
	}
	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("chat response contains no choices")
	}
	answer := strings.TrimSpace(payload.Choices[0].Message.Content)
	if answer == "" {
		return "", fmt.Errorf("chat model returned an empty answer")
	}
	return answer, nil
}

func (c *client) completeAnthropic(ctx context.Context, system, user string) (string, error) {
	headers := map[string]string{"anthropic-version": "2023-06-01"}
	if c.apiKey != "" {
		headers["x-api-key"] = c.apiKey
	}
	body, err := c.post(ctx, joinURL(c.baseURL, "messages"), headers, map[string]any{
		"model":      c.model,
		"system":     system,
		"max_tokens": c.maxTokens,
		"messages":   []map[string]string{{"role": "user", "content": user}},
	})
	if err != nil {
		return "", err
	}
	var payload struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	var parts []string
	for _, block := range payload.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	answer := strings.TrimSpace(strings.Join(parts, "\n"))
	if answer == "" {
		return "", fmt.Errorf("chat model returned an empty answer")
	}
	return answer, nil
}

func (c *client) completeOllama(ctx context.Context, system, user string) (string, error) {
	headers := map[string]string{}
	if c.apiKey != "" {
		headers["Authorization"] = "Bearer " + c.apiKey
	}
	endpoint := joinURL(c.baseURL, "api", "chat")
	if strings.HasSuffix(strings.TrimRight(c.baseURL, "/"), "/api") {
		endpoint = joinURL(c.baseURL, "chat")
	}
	body, err := c.post(ctx, endpoint, headers, map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"stream": false,
		"options": map[string]any{
			"temperature": c.temperature,
			"num_predict": c.maxTokens,
		},
	})
	if err != nil {
		return "", err
	}
	var payload struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	answer := strings.TrimSpace(payload.Message.Content)
	if answer == "" {
		return "", fmt.Errorf("chat model returned an empty answer")
	}
	return answer, nil
}

func (c *client) post(ctx context.Context, endpoint string, headers map[string]string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode chat request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read chat response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned HTTP %d: %s", c.provider, response.StatusCode, describeError(data))
	}
	return data, nil
}

func describeError(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if message, ok := payload["message"].(string); ok {
			return truncate(message, 300)
		}
		if errObject, ok := payload["error"].(map[string]any); ok {
			if message, ok := errObject["message"].(string); ok {
				return truncate(message, 300)
			}
		}
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		return text[:300]
	}
	if text == "" {
		return "no response body"
	}
	return text
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func joinURL(base string, parts ...string) string {
	out := strings.TrimRight(base, "/")
	for _, part := range parts {
		out += "/" + strings.TrimLeft(part, "/")
	}
	return out
}
