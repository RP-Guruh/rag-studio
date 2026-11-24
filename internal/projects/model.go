package projects

import "time"

// Project represents a RAG workspace.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`

	// Enriched fields for UI
	KnowledgeFilesCount int           `json:"knowledgeFilesCount"`
	PersonaName         string        `json:"personaName"`
	LastUpdated         string        `json:"lastUpdated"`
	Persona             PersonaConfig `json:"persona"`
	Documents           []Document    `json:"documents"`
}

// PersonaConfig represents the system persona for a project (1:1).
type PersonaConfig struct {
	ProjectID       string    `json:"projectId,omitempty"`
	Name            string    `json:"name"`
	Role            string    `json:"role"`
	Instructions    string    `json:"instructions"`
	Tone            string    `json:"tone"`
	AnswerLanguage  string    `json:"answerLanguage"`
	StrictKnowledge bool      `json:"strictKnowledge"`
	UpdatedAt       time.Time `json:"updatedAt,omitempty"`
}

// Document represents an uploaded document file in Knowledge Base.
type Document struct {
	ID                 string     `json:"id"`
	ProjectID          string     `json:"projectId"`
	Filename           string     `json:"filename"`
	MimeType           string     `json:"mimeType"`
	SizeBytes          int64      `json:"sizeBytes"`
	SHA256             string     `json:"sha256"`
	StoragePath        string     `json:"storagePath"`
	Status             string     `json:"status"`
	ErrorMessage       string     `json:"errorMessage,omitempty"`
	EmbeddingModel     string     `json:"embeddingModel,omitempty"`
	EmbeddingDimension int        `json:"embeddingDimension,omitempty"`
	ChunkCount         int        `json:"chunkCount"`
	CreatedAt          time.Time  `json:"createdAt"`
	IndexedAt          *time.Time `json:"indexedAt,omitempty"`
}

// DocumentInput is a file sent from the UI for import into a project.
// ContentBase64 carries the file bytes so both click-to-upload and drag & drop
// share a single code path.
type DocumentInput struct {
	Filename      string `json:"filename"`
	MimeType      string `json:"mimeType"`
	ContentBase64 string `json:"contentBase64"`
}

// Chunk is one embedded piece of a document, ready to be persisted.
type Chunk struct {
	DocumentID string         `json:"documentId"`
	ProjectID  string         `json:"projectId"`
	ChunkIndex int            `json:"chunkIndex"`
	Content    string         `json:"content"`
	TokenCount int            `json:"tokenCount"`
	PageNumber int            `json:"pageNumber"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Embedding  []float32      `json:"embedding"`
}

// ChunkHit is a retrieved chunk with its source and similarity score.
type ChunkHit struct {
	ChunkID      int64          `json:"chunkId"`
	DocumentID   string         `json:"documentId"`
	DocumentName string         `json:"documentName"`
	ChunkIndex   int            `json:"chunkIndex"`
	Content      string         `json:"content"`
	PageNumber   int            `json:"pageNumber"`
	Score        float64        `json:"score"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// ChatSource identifies a document cited by a generated answer.
type ChatSource struct {
	DocumentID string `json:"documentId"`
	FileName   string `json:"fileName"`
}

// ChatAnswer is the complete Playground response: the generated answer plus
// the evidence used to produce it.
type ChatAnswer struct {
	ProjectID string       `json:"projectId"`
	Answer    string       `json:"answer"`
	Sources   []ChatSource `json:"sources"`
	Chunks    []ChunkHit   `json:"chunks"`
	Debug     ChatDebug    `json:"debug"`
}

// ChatDebug exposes what was actually sent to the model, for testing.
type ChatDebug struct {
	SystemPrompt string `json:"systemPrompt"`
	LatencyMs    int64  `json:"latencyMs"`
	TopK         int    `json:"topK"`
}

// RetrievalConfig controls how chunks are fetched for a project.
type RetrievalConfig struct {
	TopK         int     `json:"topK"`
	MinScore     float64 `json:"minScore"`
	Hybrid       bool    `json:"hybrid"`
	Rerank       bool    `json:"rerank"`
	Guardrails   bool    `json:"guardrails"`
	HybridWeight float64 `json:"hybridWeight"`
	RerankWeight float64 `json:"rerankWeight"`
}

// EvalSet groups evaluation cases for one project.
type EvalSet struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
}

// EvalCase is one test question with the expected answer/source.
type EvalCase struct {
	ID                 string `json:"id"`
	SetID              string `json:"setId"`
	Question           string `json:"question"`
	ExpectedAnswer     string `json:"expectedAnswer"`
	ExpectedSourceName string `json:"expectedSourceName"`
}

// EvalCaseResult is the outcome of one evaluation case.
type EvalCaseResult struct {
	Question      string       `json:"question"`
	Answer        string       `json:"answer"`
	Sources       []ChatSource `json:"sources"`
	AnswerMatched bool         `json:"answerMatched"`
	SourceMatched bool         `json:"sourceMatched"`
	NoEvidence    bool         `json:"noEvidence"`
}

// ExportedDocument carries a document's metadata and file bytes for transfer.
type ExportedDocument struct {
	Document      Document `json:"document"`
	ContentBase64 string   `json:"contentBase64"`
}

// ProjectExport is the portable representation of a knowledge base.
type ProjectExport struct {
	Version   int                `json:"version"`
	Project   Project            `json:"project"`
	Documents []ExportedDocument `json:"documents"`
	Chunks    []Chunk            `json:"chunks"`
}

// EvalReport summarizes an evaluation run.
type EvalReport struct {
	SetID     string           `json:"setId"`
	Total     int              `json:"total"`
	Answered  int              `json:"answered"`
	AnswerHit int              `json:"answerHit"`
	SourceHit int              `json:"sourceHit"`
	Results   []EvalCaseResult `json:"results"`
}
