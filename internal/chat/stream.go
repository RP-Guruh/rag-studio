package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Stream generates an answer incrementally. Each non-empty token is passed to
// onToken; the complete answer is returned when the stream ends.
func (c *client) Stream(ctx context.Context, system, user string, onToken func(string)) (string, error) {
	switch c.provider {
	case "openai-compatible":
		return c.streamOpenAI(ctx, system, user, onToken)
	case "anthropic":
		return c.streamAnthropic(ctx, system, user, onToken)
	case "ollama":
		return c.streamOllama(ctx, system, user, onToken)
	default:
		return "", fmt.Errorf("unsupported chat provider %q", c.provider)
	}
}

func (c *client) streamRequest(ctx context.Context, endpoint string, headers map[string]string, body any) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode stream request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", endpoint, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		data := make([]byte, 4096)
		n, _ := response.Body.Read(data)
		return nil, fmt.Errorf("%s returned HTTP %d: %s", c.provider, response.StatusCode, describeError(data[:n]))
	}
	return response, nil
}

func (c *client) streamOpenAI(ctx context.Context, system, user string, onToken func(string)) (string, error) {
	headers := map[string]string{}
	if c.apiKey != "" {
		headers["Authorization"] = "Bearer " + c.apiKey
	}
	response, err := c.streamRequest(ctx, joinURL(c.baseURL, "chat/completions"), headers, map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": c.temperature,
		"max_tokens":  c.maxTokens,
		"stream":      true,
	})
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var answer strings.Builder
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil || len(chunk.Choices) == 0 {
			continue
		}
		emit(&answer, chunk.Choices[0].Delta.Content, onToken)
	}
	return finishStream(answer.String(), scanner.Err())
}

func (c *client) streamAnthropic(ctx context.Context, system, user string, onToken func(string)) (string, error) {
	headers := map[string]string{"anthropic-version": "2023-06-01"}
	if c.apiKey != "" {
		headers["x-api-key"] = c.apiKey
	}
	response, err := c.streamRequest(ctx, joinURL(c.baseURL, "messages"), headers, map[string]any{
		"model": c.model, "system": system, "max_tokens": c.maxTokens,
		"messages": []map[string]string{{"role": "user", "content": user}},
		"stream":   true,
	})
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var answer strings.Builder
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var chunk struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Type == "content_block_delta" && chunk.Delta.Type == "text_delta" {
			emit(&answer, chunk.Delta.Text, onToken)
		}
	}
	return finishStream(answer.String(), scanner.Err())
}

func (c *client) streamOllama(ctx context.Context, system, user string, onToken func(string)) (string, error) {
	headers := map[string]string{}
	if c.apiKey != "" {
		headers["Authorization"] = "Bearer " + c.apiKey
	}
	endpoint := joinURL(c.baseURL, "api", "chat")
	if strings.HasSuffix(strings.TrimRight(c.baseURL, "/"), "/api") {
		endpoint = joinURL(c.baseURL, "chat")
	}
	response, err := c.streamRequest(ctx, endpoint, headers, map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"stream": true,
		"options": map[string]any{
			"temperature": c.temperature,
			"num_predict": c.maxTokens,
		},
	})
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var answer strings.Builder
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		emit(&answer, chunk.Message.Content, onToken)
		if chunk.Done {
			break
		}
	}
	return finishStream(answer.String(), scanner.Err())
}

func emit(answer *strings.Builder, token string, onToken func(string)) {
	if token == "" {
		return
	}
	answer.WriteString(token)
	if onToken != nil {
		onToken(token)
	}
}

func finishStream(answer string, err error) (string, error) {
	if err != nil {
		return answer, fmt.Errorf("read stream: %w", err)
	}
	return strings.TrimSpace(answer), nil
}
