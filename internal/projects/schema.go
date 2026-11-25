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

-- Versioned schema history. Applied together with the DDL above.
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO schema_migrations (version, name) VALUES (1, 'initial rag schema')
    ON CONFLICT (version) DO NOTHING;
INSERT INTO schema_migrations (version, name) VALUES (2, 'auth and evaluation')
    ON CONFLICT (version) DO NOTHING;

-- Small key/value store for application-wide switches.
CREATE TABLE IF NOT EXISTS app_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Accounts and project assignments.
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL CHECK (role IN ('admin','user')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin','user'));
DELETE FROM users WHERE role = 'guest';
CREATE TABLE IF NOT EXISTS user_projects (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, project_id)
);

-- Retrieval tuning per project.
CREATE TABLE IF NOT EXISTS project_retrieval (
    project_id UUID PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    top_k      INTEGER NOT NULL DEFAULT 8 CHECK (top_k BETWEEN 1 AND 20),
    min_score  REAL    NOT NULL DEFAULT 0,
    hybrid     BOOLEAN NOT NULL DEFAULT false,
    rerank     BOOLEAN NOT NULL DEFAULT false,
    guardrails    BOOLEAN NOT NULL DEFAULT false,
    hybrid_weight REAL    NOT NULL DEFAULT 0.25,
    rerank_weight REAL    NOT NULL DEFAULT 0.25
);
ALTER TABLE project_retrieval ADD COLUMN IF NOT EXISTS guardrails BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE project_retrieval ADD COLUMN IF NOT EXISTS hybrid_weight REAL NOT NULL DEFAULT 0.25;
ALTER TABLE project_retrieval ADD COLUMN IF NOT EXISTS rerank_weight REAL NOT NULL DEFAULT 0.25;

-- Reusable persona instruction templates.
CREATE TABLE IF NOT EXISTS prompt_templates (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL UNIQUE,
    instructions TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO prompt_templates (name, instructions) VALUES
    ('Concise analyst', 'Answer directly and briefly. Use bullet points for lists. Cite the document names you used.')
    ON CONFLICT (name) DO NOTHING;
INSERT INTO prompt_templates (name, instructions) VALUES
    ('Support agent', 'Be friendly and helpful. Answer step by step when the question asks how to do something. If the knowledge base does not cover it, say so.')
    ON CONFLICT (name) DO NOTHING;

-- Evaluation sets used to verify that a project answers from its knowledge.
CREATE TABLE IF NOT EXISTS eval_sets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS eval_cases (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    set_id               UUID NOT NULL REFERENCES eval_sets(id) ON DELETE CASCADE,
    question             TEXT NOT NULL,
    expected_answer      TEXT DEFAULT '',
    expected_source_name TEXT DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
`, vectorDimension)
}
