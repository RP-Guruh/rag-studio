package settings

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// ValidateChatModel checks a chat model configuration before it is persisted
// or used for a connection test.
func ValidateChatModel(in ChatModel) error {
	if !slices.Contains(chatProviders, in.Provider) {
		return fmt.Errorf("unsupported chat provider %q", in.Provider)
	}
	if err := validateBaseURL(in.BaseURL); err != nil {
		return err
	}
	if strings.TrimSpace(in.ModelName) == "" {
		return fmt.Errorf("model name is required")
	}
	if in.Temperature < 0 || in.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0")
	}
	if in.MaxOutputTokens <= 0 {
		return fmt.Errorf("max output tokens must be greater than 0")
	}
	return nil
}

// ValidateEmbeddingModel checks an embedding model configuration, including
// the chunking options that are stored alongside it.
func ValidateEmbeddingModel(in EmbeddingModel) error {
	if err := validateEmbeddingEndpoint(in); err != nil {
		return err
	}
	if in.ChunkMaxTokens < 1 || in.ChunkMaxTokens > 8192 {
		return fmt.Errorf("chunk max tokens must be between 1 and 8192")
	}
	if strings.TrimSpace(in.Tokenizer) == "" {
		return fmt.Errorf("chunker tokenizer is required")
	}
	return nil
}

// validateEmbeddingEndpoint checks only the fields needed to reach the
// embedding API. Connection tests do not depend on chunking options.
func validateEmbeddingEndpoint(in EmbeddingModel) error {
	if !slices.Contains(embeddingProviders, in.Provider) {
		return fmt.Errorf("unsupported embedding provider %q", in.Provider)
	}
	if err := validateBaseURL(in.BaseURL); err != nil {
		return err
	}
	if strings.TrimSpace(in.ModelName) == "" {
		return fmt.Errorf("model name is required")
	}
	if in.VectorDimension <= 0 {
		return fmt.Errorf("vector dimension must be greater than 0")
	}
	return nil
}

// ValidateDatabase checks a PostgreSQL configuration.
func ValidateDatabase(in Database) error {
	if strings.TrimSpace(in.Host) == "" {
		return fmt.Errorf("host is required")
	}
	if in.Port < 1 || in.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if strings.TrimSpace(in.DatabaseName) == "" {
		return fmt.Errorf("database name is required")
	}
	if strings.TrimSpace(in.Username) == "" {
		return fmt.Errorf("username is required")
	}
	if !slices.Contains(sslModes, in.SSLMode) {
		return fmt.Errorf("unsupported SSL mode %q", in.SSLMode)
	}
	if strings.ContainsAny(in.SchemaName, " \t\r\n\"';") {
		return fmt.Errorf("schema name contains invalid characters")
	}
	return nil
}

// ValidateDocling checks a Docling connection configuration. Both modes reach
// Docling over HTTP, so a base URL is always required.
func ValidateDocling(in DoclingConnection) error {
	if !slices.Contains(doclingModes, in.Mode) {
		return fmt.Errorf("unsupported Docling mode %q", in.Mode)
	}
	if err := validateBaseURL(in.BaseURL); err != nil {
		return err
	}
	if in.TimeoutSeconds < 5 || in.TimeoutSeconds > 3600 {
		return fmt.Errorf("timeout must be between 5 and 3600 seconds")
	}
	return nil
}

func validateBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("base URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("base URL is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base URL must start with http:// or https://")
	}
	if u.Host == "" {
		return fmt.Errorf("base URL must include a host")
	}
	return nil
}
