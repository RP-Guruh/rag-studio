// Package embedding creates vectors for document chunks and queries using the
// embedding provider configured in Settings. It owns all HTTP knowledge of the
// supported embedding APIs.
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"rag_project/internal/settings"
)

const (
	maxResponseBytes   = 8 << 20
	defaultBatchSize   = 32
	defaultHTTPTimeout = 120 * time.Second
)

// Embedder converts texts into vectors of a fixed dimension.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
	Model() string
}

type client struct {
	provider  string
	baseURL   string
	model     string
	apiKey    string
	dimension int
	batchSize int
	http      *http.Client

	mu    sync.Mutex
	cache map[string][]float32
}

// New builds an Embedder from the saved embedding settings. No defaults are
// substituted; every value must have been configured and saved by the user.
func New(cfg settings.EmbeddingModel, apiKey string) (Embedder, error) {
	if err := settings.ValidateEmbeddingModel(cfg); err != nil {
		return nil, err
	}
	return &client{
		provider:  settings.NormalizeProvider(cfg.Provider),
		baseURL:   strings.TrimSpace(cfg.BaseURL),
		model:     strings.TrimSpace(cfg.ModelName),
		apiKey:    strings.TrimSpace(apiKey),
		dimension: cfg.VectorDimension,
		batchSize: defaultBatchSize,
		http:      &http.Client{Timeout: defaultHTTPTimeout},
		cache:     map[string][]float32{},
	}, nil
}

func (c *client) Dimension() int { return c.dimension }
func (c *client) Model() string  { return c.model }

// Embed embeds every text, keeping the input order. It fails if the endpoint
// returns a different number of vectors or a vector of the wrong dimension,
// so a partially indexed document is never persisted.
func (c *client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	result := make([][]float32, len(texts))
	missing := []string{}
	missingAt := []int{}

	c.mu.Lock()
	for i, text := range texts {
		if cached, ok := c.cache[text]; ok {
			result[i] = cached
		} else {
			missing = append(missing, text)
			missingAt = append(missingAt, i)
		}
	}
	c.mu.Unlock()

	for start := 0; start < len(missing); start += c.batchSize {
		end := start + c.batchSize
		if end > len(missing) {
			end = len(missing)
		}
		vectors, err := c.embedBatch(ctx, missing[start:end])
		if err != nil {
			return nil, fmt.Errorf("embed inputs %d-%d: %w", start+1, end, err)
		}
		if len(vectors) != end-start {
			return nil, fmt.Errorf("embedding endpoint returned %d vectors for %d inputs", len(vectors), end-start)
		}
		for i, vector := range vectors {
			if len(vector) == 0 {
				return nil, fmt.Errorf("embedding for input %d is empty", missingAt[start+i]+1)
			}
			if len(vector) != c.dimension {
				return nil, fmt.Errorf("embedding for input %d has %d dimensions, expected %d", missingAt[start+i]+1, len(vector), c.dimension)
			}
			result[missingAt[start+i]] = vector
		}
	}

	c.mu.Lock()
	for i, text := range missing {
		c.cache[text] = result[missingAt[i]]
	}
	c.mu.Unlock()
	return result, nil
}

func (c *client) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	switch c.provider {
	case "openai-compatible":
		return c.embedOpenAI(ctx, texts)
	case "ollama":
		return c.embedOllama(ctx, texts)
	case "huggingface":
		return c.embedHuggingFace(ctx, texts)
	default:
		return nil, fmt.Errorf("unsupported embedding provider %q", c.provider)
	}
}

func (c *client) embedOpenAI(ctx context.Context, texts []string) ([][]float32, error) {
	headers := map[string]string{}
	if c.apiKey != "" {
		headers["Authorization"] = "Bearer " + c.apiKey
	}
	body, err := c.post(ctx, joinURL(c.baseURL, "embeddings"), headers, map[string]any{
		"model": c.model,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("response contains no embedding data")
	}
	sort.SliceStable(payload.Data, func(i, j int) bool {
		return payload.Data[i].Index < payload.Data[j].Index
	})
	out := make([][]float32, 0, len(payload.Data))
	for _, item := range payload.Data {
		out = append(out, item.Embedding)
	}
	return out, nil
}

func (c *client) embedOllama(ctx context.Context, texts []string) ([][]float32, error) {
	headers := map[string]string{}
	if c.apiKey != "" {
		headers["Authorization"] = "Bearer " + c.apiKey
	}
	endpoint := joinURL(c.baseURL, "api", "embed")
	if strings.HasSuffix(strings.TrimRight(c.baseURL, "/"), "/api") {
		endpoint = joinURL(c.baseURL, "embed")
	}
	body, err := c.post(ctx, endpoint, headers, map[string]any{
		"model": c.model,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}
	if len(payload.Embeddings) == 0 {
		return nil, fmt.Errorf("response contains no embeddings")
	}
	return payload.Embeddings, nil
}

func (c *client) embedHuggingFace(ctx context.Context, texts []string) ([][]float32, error) {
	headers := map[string]string{}
	if c.apiKey != "" {
		headers["Authorization"] = "Bearer " + c.apiKey
	}
	body, err := c.post(ctx, joinURL(c.baseURL, "models", c.model), headers, map[string]any{
		"inputs":  texts,
		"options": map[string]any{"wait_for_model": true},
	})
	if err != nil {
		return nil, err
	}
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}
	return parseHuggingFaceVectors(raw, len(texts))
}

// parseHuggingFaceVectors handles the response shapes returned by the
// Inference API: one vector per input, token vectors per input (mean-pooled),
// or a single vector when only one input was sent.
func parseHuggingFaceVectors(raw any, inputs int) ([][]float32, error) {
	outer, ok := raw.([]any)
	if !ok || len(outer) == 0 {
		return nil, fmt.Errorf("unexpected embedding response shape")
	}

	// Single vector: [0.1, 0.2, ...]
	if _, ok := outer[0].(float64); ok {
		if inputs != 1 {
			return nil, fmt.Errorf("endpoint returned one vector for %d inputs", inputs)
		}
		vector, err := floatSlice(outer)
		if err != nil {
			return nil, err
		}
		return [][]float32{vector}, nil
	}

	first, ok := outer[0].([]any)
	if !ok || len(first) == 0 {
		return nil, fmt.Errorf("unexpected embedding response shape")
	}

	// One vector per input: [[0.1, 0.2], [0.3, 0.4]]
	if _, ok := first[0].(float64); ok {
		out := make([][]float32, 0, len(outer))
		for _, item := range outer {
			vector, err := floatSlice(item)
			if err != nil {
				return nil, err
			}
			out = append(out, vector)
		}
		return out, nil
	}

	// Token vectors per input: [[[t1...], [t2...]], ...]
	out := make([][]float32, 0, len(outer))
	for _, item := range outer {
		tokens, ok := item.([]any)
		if !ok || len(tokens) == 0 {
			return nil, fmt.Errorf("unexpected embedding response shape")
		}
		vectors := make([][]float32, 0, len(tokens))
		for _, token := range tokens {
			vector, err := floatSlice(token)
			if err != nil {
				return nil, err
			}
			vectors = append(vectors, vector)
		}
		out = append(out, meanPool(vectors))
	}
	return out, nil
}

func floatSlice(value any) ([]float32, error) {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("unexpected vector shape")
	}
	out := make([]float32, 0, len(items))
	for _, item := range items {
		number, ok := item.(float64)
		if !ok {
			return nil, fmt.Errorf("vector contains a non-numeric value")
		}
		out = append(out, float32(number))
	}
	return out, nil
}

func meanPool(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}
	dims := len(vectors[0])
	out := make([]float32, dims)
	for _, vector := range vectors {
		if len(vector) != dims {
			continue
		}
		for i, value := range vector {
			out[i] += value
		}
	}
	for i := range out {
		out[i] /= float32(len(vectors))
	}
	return out
}

func (c *client) post(ctx context.Context, endpoint string, headers map[string]string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode embedding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build embedding request: %w", err)
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
		return nil, fmt.Errorf("read embedding response: %w", err)
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
