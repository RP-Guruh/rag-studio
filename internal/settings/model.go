package settings

import "strings"

// ChatModel is the configuration of the LLM used to synthesize answers.
type ChatModel struct {
	Provider        string  `json:"provider"`
	BaseURL         string  `json:"baseUrl"`
	ModelName       string  `json:"modelName"`
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`

	APIKey      string `json:"apiKey,omitempty"`
	ClearAPIKey bool   `json:"clearApiKey,omitempty"`
	APIKeySet   bool   `json:"apiKeySet"`
	APIKeyHint  string `json:"apiKeyHint"`

	// Configured is true only when the user has saved this section.
	Configured bool `json:"configured"`

	UpdatedAt string `json:"updatedAt,omitempty"`
}

// EmbeddingModel is the configuration of the vector embedding model.
type EmbeddingModel struct {
	Provider        string `json:"provider"`
	BaseURL         string `json:"baseUrl"`
	ModelName       string `json:"modelName"`
	VectorDimension int    `json:"vectorDimension"`

	// Chunking options used by the document pipeline. They live here because
	// chunk boundaries must match the embedding model's tokenizer and limits.
	ChunkMaxTokens int    `json:"chunkMaxTokens"`
	Tokenizer      string `json:"tokenizer"`

	APIKey      string `json:"apiKey,omitempty"`
	ClearAPIKey bool   `json:"clearApiKey,omitempty"`
	APIKeySet   bool   `json:"apiKeySet"`
	APIKeyHint  string `json:"apiKeyHint"`

	// Configured is true only when the user has saved this section.
	Configured bool `json:"configured"`

	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Database is the PostgreSQL connection used for vector storage.
type Database struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DatabaseName string `json:"databaseName"`
	Username     string `json:"username"`
	SSLMode      string `json:"sslMode"`
	SchemaName   string `json:"schemaName"`

	Password      string `json:"password,omitempty"`
	ClearPassword bool   `json:"clearPassword,omitempty"`
	PasswordSet   bool   `json:"passwordSet"`
	PasswordHint  string `json:"passwordHint"`

	// Configured is true only when the user has saved this section.
	Configured bool `json:"configured"`

	UpdatedAt string `json:"updatedAt,omitempty"`
}

// DoclingConnection is the Docling document-processing engine configuration.
type DoclingConnection struct {
	Mode           string `json:"mode"`
	BaseURL        string `json:"baseUrl"`
	TimeoutSeconds int    `json:"timeoutSeconds"`

	APIKey      string `json:"apiKey,omitempty"`
	ClearAPIKey bool   `json:"clearApiKey,omitempty"`
	APIKeySet   bool   `json:"apiKeySet"`
	APIKeyHint  string `json:"apiKeyHint"`

	// Configured is true only when the user has saved this section.
	Configured bool `json:"configured"`

	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Settings groups the configuration sections.
type Settings struct {
	ChatModel      ChatModel         `json:"chatModel"`
	EmbeddingModel EmbeddingModel    `json:"embeddingModel"`
	Database       Database          `json:"database"`
	Docling        DoclingConnection `json:"docling"`
}

// TestResult is the outcome of a connection test.
type TestResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	LatencyMs int64  `json:"latencyMs"`
}

// SecretState holds the set/clear state of a secret for display/hint purposes.
type SecretState struct {
	SetFlag bool   `json:"set"`
	HintVal string `json:"hint"`
}

// NewSecretState creates a SecretState.
func NewSecretState(set bool, hint string) SecretState {
	return SecretState{SetFlag: set, HintVal: hint}
}

// Set returns the set flag status.
func (s SecretState) Set() bool { return s.SetFlag }

// Hint returns the hint string.
func (s SecretState) Hint() string { return s.HintVal }

// String constants for secret store.
const (
	secretChatModel      = "chat_model_api_key"
	secretEmbeddingModel = "embedding_model_api_key"
	secretDatabase       = "database_password"
	secretDocling        = "docling_api_key"
)

// Supported providers. Validated in Go instead of a SQLite CHECK constraint so
// adding a provider does not require rebuilding the table.
var (
	chatProviders      = []string{"openai-compatible", "anthropic", "ollama"}
	embeddingProviders = []string{"openai-compatible", "ollama", "huggingface"}
	sslModes           = []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
	doclingModes       = []string{"local", "serve"}
)

// Repository is the interface for settings data operations.
type Repository interface {
	Get() (Settings, error)
	GetChatModel() (ChatModel, error)
	GetEmbeddingModel() (EmbeddingModel, error)
	GetDatabase() (Database, error)
	GetDocling() (DoclingConnection, error)
	SaveChatModel(ChatModel) (ChatModel, error)
	SaveEmbeddingModel(EmbeddingModel) (EmbeddingModel, error)
	SaveDatabase(Database) (Database, error, bool)
	SaveDocling(DoclingConnection) (DoclingConnection, error)
	ChatModelAPIKey() (string, error)
	EmbeddingAPIKey() (string, error)
	DatabasePassword() (string, error)
	DoclingAPIKey() (string, error)
}

// NormalizeProvider normalizes a provider name to lowercase.
func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}
