package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	probeTimeout      = 20 * time.Second
	maxResponseBytes  = 64 << 10
	probeChatPrompt   = "Reply with the single word: ok"
	probeEmbeddingTxt = "ping"
)

// TestChatModel performs a minimal round-trip against the configured chat
// provider. apiKey may be empty to test without authentication (for example
// Ollama).
func TestChatModel(ctx context.Context, in ChatModel, apiKey string) (TestResult, error) {
	in.Provider = normalizeProvider(in.Provider)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.ModelName = strings.TrimSpace(in.ModelName)
	if err := ValidateChatModel(in); err != nil {
		return TestResult{}, err
	}

	headers := map[string]string{}
	var (
		endpoint string
		body     any
	)

	switch in.Provider {
	case "openai-compatible":
		endpoint = joinURL(in.BaseURL, "chat/completions")
		body = map[string]any{
			"model":      in.ModelName,
			"messages":   []map[string]string{{"role": "user", "content": probeChatPrompt}},
			"max_tokens": 1,
			"stream":     false,
		}
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
	case "anthropic":
		endpoint = joinURL(in.BaseURL, "messages")
		body = map[string]any{
			"model":      in.ModelName,
			"max_tokens": 1,
			"messages":   []map[string]string{{"role": "user", "content": probeChatPrompt}},
		}
		headers["anthropic-version"] = "2023-06-01"
		if apiKey != "" {
			headers["x-api-key"] = apiKey
		}
	case "ollama":
		endpoint = ollamaEndpoint(in.BaseURL, "chat")
		body = map[string]any{
			"model":    in.ModelName,
			"messages": []map[string]string{{"role": "user", "content": probeChatPrompt}},
			"stream":   false,
		}
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
	default:
		return TestResult{}, fmt.Errorf("unsupported chat provider %q", in.Provider)
	}

	start := time.Now()
	status, response, err := postJSON(ctx, endpoint, headers, body)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{Message: fmt.Sprintf("Could not reach %s: %v", endpoint, err), LatencyMs: latency}, nil
	}
	if status < 200 || status >= 300 {
		return TestResult{
			Message:   fmt.Sprintf("%s returned HTTP %d: %s", in.Provider, status, describeAPIError(response)),
			LatencyMs: latency,
		}, nil
	}

	return TestResult{
		Success:   true,
		Message:   fmt.Sprintf("Connected to %s (%s). Roundtrip: %d ms.", in.Provider, in.ModelName, latency),
		LatencyMs: latency,
	}, nil
}

// TestEmbeddingModel calls the embedding endpoint once and verifies that the
// returned vector length matches the configured dimension.
func TestEmbeddingModel(ctx context.Context, in EmbeddingModel, apiKey string) (TestResult, error) {
	in.Provider = normalizeProvider(in.Provider)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.ModelName = strings.TrimSpace(in.ModelName)
	if err := validateEmbeddingEndpoint(in); err != nil {
		return TestResult{}, err
	}

	headers := map[string]string{}
	var (
		endpoint string
		body     any
		parse    func([]byte) ([]float64, error)
	)

	switch in.Provider {
	case "openai-compatible":
		endpoint = joinURL(in.BaseURL, "embeddings")
		body = map[string]any{"model": in.ModelName, "input": probeEmbeddingTxt}
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
		parse = parseOpenAIEmbedding
	case "ollama":
		endpoint = ollamaEndpoint(in.BaseURL, "embed")
		body = map[string]any{"model": in.ModelName, "input": probeEmbeddingTxt}
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
		parse = parseOllamaEmbedding
	case "huggingface":
		endpoint = joinURL(in.BaseURL, "models", in.ModelName)
		body = map[string]any{"inputs": probeEmbeddingTxt}
		if apiKey != "" {
			headers["Authorization"] = "Bearer " + apiKey
		}
		parse = parseHuggingFaceEmbedding
	default:
		return TestResult{}, fmt.Errorf("unsupported embedding provider %q", in.Provider)
	}

	start := time.Now()
	status, response, err := postJSON(ctx, endpoint, headers, body)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return TestResult{Message: fmt.Sprintf("Could not reach %s: %v", endpoint, err), LatencyMs: latency}, nil
	}
	if status < 200 || status >= 300 {
		return TestResult{
			Message:   fmt.Sprintf("%s returned HTTP %d: %s", in.Provider, status, describeAPIError(response)),
			LatencyMs: latency,
		}, nil
	}

	vector, err := parse(response)
	if err != nil {
		return TestResult{
			Message:   fmt.Sprintf("%s responded but the embedding could not be read: %v", in.Provider, err),
			LatencyMs: latency,
		}, nil
	}
	if in.VectorDimension > 0 && len(vector) != in.VectorDimension {
		return TestResult{
			Message: fmt.Sprintf(
				"Embedding endpoint returned %d dimensions, but the configured dimension is %d. Update the dimension or the model before saving.",
				len(vector), in.VectorDimension,
			),
			LatencyMs: latency,
		}, nil
	}

	return TestResult{
		Success:   true,
		Message:   fmt.Sprintf("Embedding model responded with %d dimensions. Roundtrip: %d ms.", len(vector), latency),
		LatencyMs: latency,
	}, nil
}

// TestDatabase connects to PostgreSQL and verifies that the pgvector extension
// is installed.
func TestDatabase(ctx context.Context, in Database, password string) (TestResult, error) {
	in.Host = strings.TrimSpace(in.Host)
	in.DatabaseName = strings.TrimSpace(in.DatabaseName)
	in.Username = strings.TrimSpace(in.Username)
	in.SSLMode = strings.ToLower(strings.TrimSpace(in.SSLMode))
	in.SchemaName = strings.TrimSpace(in.SchemaName)
	if in.SchemaName == "" {
		in.SchemaName = "public"
	}
	if err := ValidateDatabase(in); err != nil {
		return TestResult{}, err
	}

	dsn, err := postgresDSN(in, password)
	if err != nil {
		return TestResult{}, err
	}

	start := time.Now()
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return TestResult{Message: "Database settings produced an invalid connection string."}, nil
	}
	config.ConnectTimeout = 10 * time.Second

	conn, err := pgx.ConnectConfig(connectCtx, config)
	if err != nil {
		return TestResult{
			Message:   fmt.Sprintf("Could not connect to %s:%d: %s", in.Host, in.Port, redact(err.Error(), password)),
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}
	defer conn.Close(connectCtx)

	var serverVersion string
	if err := conn.QueryRow(connectCtx, "SHOW server_version").Scan(&serverVersion); err != nil {
		return TestResult{Message: fmt.Sprintf("Connected but could not read the server version: %v", err)}, nil
	}

	var vectorVersion string
	err = conn.QueryRow(connectCtx, "SELECT extversion FROM pg_extension WHERE extname = 'vector'").Scan(&vectorVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return TestResult{
			Message: fmt.Sprintf(
				"Connected to PostgreSQL %s at %s:%d, but the pgvector extension is not installed. Run: CREATE EXTENSION vector;",
				serverVersion, in.Host, in.Port,
			),
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}
	if err != nil {
		return TestResult{Message: fmt.Sprintf("Connected but could not check for pgvector: %v", err)}, nil
	}

	var schemaExists bool
	if err := conn.QueryRow(connectCtx,
		"SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)", in.SchemaName,
	).Scan(&schemaExists); err != nil {
		return TestResult{Message: fmt.Sprintf("Connected but could not check the schema: %v", err)}, nil
	}
	if !schemaExists {
		return TestResult{
			Message: fmt.Sprintf(
				"Connected to PostgreSQL %s, but schema %q does not exist. Create it or choose another schema.",
				serverVersion, in.SchemaName,
			),
			LatencyMs: time.Since(start).Milliseconds(),
		}, nil
	}

	return TestResult{
		Success: true,
		Message: fmt.Sprintf(
			"PostgreSQL %s connected at %s:%d; schema %q and pgvector %s are ready. Roundtrip: %d ms.",
			serverVersion, in.Host, in.Port, in.SchemaName, vectorVersion, time.Since(start).Milliseconds(),
		),
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// TestDocling verifies the document-processing engine by calling the
// docling-serve health endpoint. Both modes reach Docling over HTTP: "local"
// means the service runs on this machine, "serve" means it runs elsewhere.
func TestDocling(ctx context.Context, in DoclingConnection, apiKey string) (TestResult, error) {
	in.Mode = strings.ToLower(strings.TrimSpace(in.Mode))
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	if err := ValidateDocling(in); err != nil {
		return TestResult{}, err
	}

	endpoint := joinURL(in.BaseURL, "health")
	headers := map[string]string{}
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}

	start := time.Now()
	status, response, err := getJSON(ctx, endpoint, headers)
	latency := time.Since(start).Milliseconds()
	label := "Docling"
	if in.Mode == "local" {
		label = "Local Docling"
	}
	if err != nil {
		return TestResult{
			Message: fmt.Sprintf(
				"Could not reach %s at %s: %v. Check that docling-serve is running and the base URL is correct.",
				label, endpoint, err,
			),
			LatencyMs: latency,
		}, nil
	}
	if status < 200 || status >= 300 {
		return TestResult{
			Message:   fmt.Sprintf("%s returned HTTP %d: %s", label, status, describeAPIError(response)),
			LatencyMs: latency,
		}, nil
	}

	message := fmt.Sprintf("%s is reachable at %s. Roundtrip: %d ms.", label, in.BaseURL, latency)
	if version := doclingVersionFromHealth(response); version != "" {
		message = fmt.Sprintf("%s %s is reachable at %s. Roundtrip: %d ms.", label, version, in.BaseURL, latency)
	}
	return TestResult{Success: true, Message: message, LatencyMs: latency}, nil
}

// doclingVersionFromHealth reads an optional version field from /health.
func doclingVersionFromHealth(body []byte) string {
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Version)
}

func getJSON(ctx context.Context, endpoint string, headers map[string]string) (int, []byte, error) {
	requestCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return response.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return response.StatusCode, data, nil
}

func postJSON(ctx context.Context, endpoint string, headers map[string]string, body any) (int, []byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, nil, fmt.Errorf("encode request: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return response.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return response.StatusCode, data, nil
}

func describeAPIError(body []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if raw, ok := payload["error"]; ok {
			switch value := raw.(type) {
			case string:
				return truncate(value, 300)
			case map[string]any:
				if message, ok := value["message"].(string); ok {
					return truncate(message, 300)
				}
			}
		}
		if message, ok := payload["message"].(string); ok {
			return truncate(message, 300)
		}
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "no response body"
	}
	return truncate(text, 300)
}

func parseOpenAIEmbedding(body []byte) ([]float64, error) {
	var payload struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("response contains no embedding data")
	}
	return payload.Data[0].Embedding, nil
}

func parseOllamaEmbedding(body []byte) ([]float64, error) {
	var payload struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if len(payload.Embeddings) == 0 {
		return nil, fmt.Errorf("response contains no embeddings")
	}
	return payload.Embeddings[0], nil
}

func parseHuggingFaceEmbedding(body []byte) ([]float64, error) {
	var single []float64
	if err := json.Unmarshal(body, &single); err == nil {
		return single, nil
	}
	var batched [][]float64
	if err := json.Unmarshal(body, &batched); err != nil {
		return nil, err
	}
	if len(batched) == 0 {
		return nil, fmt.Errorf("response contains no embeddings")
	}
	return batched[0], nil
}

func joinURL(base string, parts ...string) string {
	out := strings.TrimRight(base, "/")
	for _, part := range parts {
		out += "/" + strings.TrimLeft(part, "/")
	}
	return out
}

// ollamaEndpoint avoids a duplicated /api prefix when the user already included
// it in the base URL.
func ollamaEndpoint(base, path string) string {
	if strings.HasSuffix(strings.TrimRight(base, "/"), "/api") {
		return joinURL(base, path)
	}
	return joinURL(base, "api", path)
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func redact(message, secret string) string {
	if secret == "" {
		return message
	}
	return strings.ReplaceAll(message, secret, "[redacted]")
}

func postgresDSN(in Database, password string) (string, error) {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(in.Username, password),
		Host:   net.JoinHostPort(in.Host, strconv.Itoa(in.Port)),
		Path:   "/" + in.DatabaseName,
	}
	query := u.Query()
	query.Set("sslmode", in.SSLMode)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

// The PostgreSQL RAG schema lives in internal/projects, which owns table
// migrations. Settings only needs connection checks here.
