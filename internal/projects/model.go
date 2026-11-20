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
