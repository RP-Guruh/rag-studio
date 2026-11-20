package projects

import "fmt"

// SchemaSQL returns the RAG schema for the configured embedding dimension.
// The dimension comes from Settings → Embedding Model; callers must validate it
// is positive before calling this function.
func SchemaSQL(vectorDimension int) string {
	return fmt.Sprintf(`
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS projects (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    description  TEXT DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS project_personas (
    project_id       UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    persona_name     TEXT NOT NULL,
    role             TEXT DEFAULT '',
    instructions     TEXT DEFAULT '',
    tone             TEXT NOT NULL DEFAULT 'neutral'
                     CHECK (tone IN ('neutral','friendly','formal','concise')),
    answer_language  TEXT NOT NULL DEFAULT 'auto',
    strict_knowledge BOOLEAN NOT NULL DEFAULT true,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS documents (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    filename             TEXT NOT NULL,
    mime_type            TEXT DEFAULT '',
    size_bytes           BIGINT DEFAULT 0,
    sha256               TEXT,
    storage_path         TEXT DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending','processing','chunked','indexed','failed')),
    error_message        TEXT,
    parse_options        JSONB,
    embedding_model      TEXT,
    embedding_dimension  INTEGER,
    chunk_count          INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    indexed_at           TIMESTAMPTZ,
    UNIQUE (project_id, sha256)
);

-- Keep databases created before the chunking pipeline in sync.
ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_status_check;
ALTER TABLE documents ADD CONSTRAINT documents_status_check
    CHECK (status IN ('pending','processing','chunked','indexed','failed'));

CREATE INDEX IF NOT EXISTS idx_documents_project ON documents(project_id);

CREATE TABLE IF NOT EXISTS chunks (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    document_id   UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    chunk_index   INTEGER NOT NULL,
    content       TEXT NOT NULL,
    token_count   INTEGER,
    page_number   INTEGER,
    metadata      JSONB,
    embedding     vector(%d) NOT NULL,
    UNIQUE (document_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_chunks_project ON chunks(project_id);
CREATE INDEX IF NOT EXISTS idx_chunks_embedding ON chunks
    USING hnsw (embedding vector_cosine_ops);
`, vectorDimension)
}
