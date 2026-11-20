package projects

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"rag_project/internal/settings"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStoreFromSettings(dbConfig settings.Database, password string) (*Store, error) {
	dsn, err := postgresDSN(dbConfig, password)
	if err != nil {
		return nil, fmt.Errorf("build postgres dsn: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("connect postgres pool: %w", err)
	}

	return &Store{pool: pool}, nil
}

// MigratePostgres applies the RAG schema using the configured embedding
// dimension and keeps an existing chunks.embedding column in sync.
func MigratePostgres(ctx context.Context, dbConfig settings.Database, password string, vectorDimension int) error {
	if vectorDimension <= 0 {
		return fmt.Errorf("vector dimension must be greater than 0")
	}
	dsn, err := postgresDSN(dbConfig, password)
	if err != nil {
		return fmt.Errorf("build postgres dsn: %w", err)
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect postgres for migration: %w", err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, SchemaSQL(vectorDimension)); err != nil {
		return fmt.Errorf("execute schema migration: %w", err)
	}
	return ensureChunksVectorDimension(ctx, conn, vectorDimension)
}

// EnsureChunksVectorDimension verifies that chunks.embedding matches the
// configured embedding dimension. The column can only be altered while the
// table is empty, which is the case during the chunking-only phase.
func (s *Store) EnsureChunksVectorDimension(ctx context.Context, vectorDimension int) error {
	if vectorDimension <= 0 {
		return fmt.Errorf("vector dimension must be greater than 0")
	}
	return ensureChunksVectorDimension(ctx, s.pool, vectorDimension)
}

// ReindexChunksVectorDimension forcibly resets chunks.embedding to the given
// dimension. It drops the existing vector column and creates a fresh one with
// the requested dimension, then recreates the HNSW index. Existing embedding
// vectors are discarded. Use this only when changing the embedding model
// dimension and the old vectors are no longer valid (for example, after
// retraining with a new model). This operation is safe because the chunking-only
// phase does not yet persist searchable embeddings to the vector index.
func (s *Store) ReindexChunksVectorDimension(ctx context.Context, vectorDimension int) error {
	if vectorDimension <= 0 {
		return fmt.Errorf("vector dimension must be greater than 0")
	}
	if s == nil || s.pool == nil {
		return fmt.Errorf("store not initialized")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reindex transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Drop the old index if it exists.
	if _, err := tx.Exec(ctx, `DROP INDEX IF EXISTS idx_chunks_embedding`); err != nil {
		return fmt.Errorf("drop chunks embedding index: %w", err)
	}

	// Drop the old embedding column.
	if _, err := tx.Exec(ctx, `ALTER TABLE chunks DROP COLUMN embedding`); err != nil {
		return fmt.Errorf("drop old embedding column: %w", err)
	}

	// Add a new embedding column with the requested dimension.
	if _, err := tx.Exec(ctx, fmt.Sprintf(
		`ALTER TABLE chunks ADD COLUMN embedding vector(%d) NOT NULL DEFAULT '[]'::vector`,
		vectorDimension,
	)); err != nil {
		return fmt.Errorf("add new embedding column: %w", err)
	}

	// Recreate the HNSW index on the new column.
	if _, err := tx.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_chunks_embedding ON chunks
			USING hnsw (embedding vector_cosine_ops)
	`); err != nil {
		return fmt.Errorf("recreate chunks embedding index: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reindex transaction: %w", err)
	}
	return nil
}

type schemaExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func ensureChunksVectorDimension(ctx context.Context, db schemaExecutor, vectorDimension int) error {
	var columnType string
	err := db.QueryRow(ctx, `
		SELECT format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a
		WHERE a.attrelid = 'chunks'::regclass
		  AND a.attname = 'embedding'
		  AND NOT a.attisdropped
	`).Scan(&columnType)
	if err != nil {
		return fmt.Errorf("inspect chunks.embedding type: %w", err)
	}

	current, ok := parseVectorDimension(columnType)
	if ok && current == vectorDimension {
		return nil
	}

	var hasRows bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM chunks)`).Scan(&hasRows); err != nil {
		return fmt.Errorf("check chunks table: %w", err)
	}
	if hasRows {
		return fmt.Errorf(
			"chunks.embedding uses %s but the configured embedding dimension is %d; re-index the documents after changing the embedding model",
			columnType, vectorDimension,
		)
	}

	if _, err := db.Exec(ctx, `DROP INDEX IF EXISTS idx_chunks_embedding`); err != nil {
		return fmt.Errorf("drop chunks embedding index: %w", err)
	}
	alter := fmt.Sprintf(
		`ALTER TABLE chunks ALTER COLUMN embedding TYPE vector(%d) USING embedding::vector(%d)`,
		vectorDimension, vectorDimension,
	)
	if _, err := db.Exec(ctx, alter); err != nil {
		return fmt.Errorf("alter chunks.embedding dimension: %w", err)
	}
	if _, err := db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_chunks_embedding ON chunks
			USING hnsw (embedding vector_cosine_ops)
	`); err != nil {
		return fmt.Errorf("recreate chunks embedding index: %w", err)
	}
	return nil
}

func parseVectorDimension(columnType string) (int, bool) {
	columnType = strings.TrimSpace(columnType)
	if !strings.HasPrefix(columnType, "vector(") || !strings.HasSuffix(columnType, ")") {
		return 0, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(columnType, "vector("), ")")
	dimension, err := strconv.Atoi(raw)
	if err != nil || dimension <= 0 {
		return 0, false
	}
	return dimension, true
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	query := `
		SELECT 
			p.id, p.name, COALESCE(p.description, ''), p.created_at, p.updated_at,
			COALESCE(pp.persona_name, 'General Assistant'),
			COALESCE(pp.role, 'Knowledge Assistant'),
			COALESCE(pp.instructions, 'You are a helpful assistant.'),
			COALESCE(pp.tone, 'neutral'),
			COALESCE(pp.answer_language, 'auto'),
			COALESCE(pp.strict_knowledge, true),
			(SELECT COUNT(*) FROM documents d WHERE d.project_id = p.id) as doc_count
		FROM projects p
		LEFT JOIN project_personas pp ON pp.project_id = p.id
		ORDER BY p.updated_at DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	var result []Project
	for rows.Next() {
		var p Project
		var pers PersonaConfig
		var docCount int
		err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt,
			&pers.Name, &pers.Role, &pers.Instructions, &pers.Tone, &pers.AnswerLanguage, &pers.StrictKnowledge,
			&docCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan project row: %w", err)
		}
		pers.ProjectID = p.ID
		p.Persona = pers
		p.PersonaName = pers.Name
		p.KnowledgeFilesCount = docCount
		p.LastUpdated = p.UpdatedAt.Format("2006-01-02")
		result = append(result, p)
	}
	if result == nil {
		result = []Project{}
	}
	return result, nil
}

func (s *Store) GetProject(ctx context.Context, id string) (*Project, error) {
	query := `
		SELECT 
			p.id, p.name, COALESCE(p.description, ''), p.created_at, p.updated_at,
			COALESCE(pp.persona_name, 'General Assistant'),
			COALESCE(pp.role, 'Knowledge Assistant'),
			COALESCE(pp.instructions, 'You are a helpful assistant.'),
			COALESCE(pp.tone, 'neutral'),
			COALESCE(pp.answer_language, 'auto'),
			COALESCE(pp.strict_knowledge, true),
			(SELECT COUNT(*) FROM documents d WHERE d.project_id = p.id) as doc_count
		FROM projects p
		LEFT JOIN project_personas pp ON pp.project_id = p.id
		WHERE p.id = $1
	`
	var p Project
	var pers PersonaConfig
	var docCount int
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt,
		&pers.Name, &pers.Role, &pers.Instructions, &pers.Tone, &pers.AnswerLanguage, &pers.StrictKnowledge,
		&docCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query project: %w", err)
	}
	pers.ProjectID = p.ID
	p.Persona = pers
	p.PersonaName = pers.Name
	p.KnowledgeFilesCount = docCount
	p.LastUpdated = p.UpdatedAt.Format("2006-01-02")

	docs, err := s.ListDocuments(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Documents = docs
	return &p, nil
}

func (s *Store) CreateProject(ctx context.Context, name, description string) (*Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("project name is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var p Project
	err = tx.QueryRow(ctx, `
		INSERT INTO projects (name, description, created_at, updated_at)
		VALUES ($1, $2, now(), now())
		RETURNING id, name, description, created_at, updated_at
	`, name, description).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}

	// Create default persona
	defaultPersonaName := name + " Assistant"
	_, err = tx.Exec(ctx, `
		INSERT INTO project_personas (project_id, persona_name, role, instructions, tone, answer_language, strict_knowledge, updated_at)
		VALUES ($1, $2, 'Knowledge Assistant', 'You are a helpful assistant. Use the uploaded files to answer questions accurately.', 'neutral', 'auto', true, now())
	`, p.ID, defaultPersonaName)
	if err != nil {
		return nil, fmt.Errorf("insert default persona: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.GetProject(ctx, p.ID)
}

func (s *Store) UpdateProject(ctx context.Context, id, name, description string) (*Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("project name cannot be empty")
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE projects SET name = $1, description = $2, updated_at = now() WHERE id = $3
	`, name, description, id)
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	return s.GetProject(ctx, id)
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if err := RemoveProjectFiles(id); err != nil {
		return fmt.Errorf("project deleted but local files could not be removed: %w", err)
	}
	return nil
}

func (s *Store) UpdatePersona(ctx context.Context, projectId string, persona PersonaConfig) error {
	tone := strings.ToLower(strings.TrimSpace(persona.Tone))
	if tone != "neutral" && tone != "friendly" && tone != "formal" && tone != "concise" {
		tone = "neutral"
	}
	name := strings.TrimSpace(persona.Name)
	if name == "" {
		name = "General Assistant"
	}
	lang := strings.TrimSpace(persona.AnswerLanguage)
	if lang == "" {
		lang = "auto"
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO project_personas (project_id, persona_name, role, instructions, tone, answer_language, strict_knowledge, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (project_id) DO UPDATE SET
			persona_name     = excluded.persona_name,
			role             = excluded.role,
			instructions     = excluded.instructions,
			tone             = excluded.tone,
			answer_language  = excluded.answer_language,
			strict_knowledge = excluded.strict_knowledge,
			updated_at       = now()
	`, projectId, name, persona.Role, persona.Instructions, tone, lang, persona.StrictKnowledge)
	if err != nil {
		return fmt.Errorf("upsert persona: %w", err)
	}

	// Update project's updated_at
	_, _ = s.pool.Exec(ctx, `UPDATE projects SET updated_at = now() WHERE id = $1`, projectId)
	return nil
}

func postgresDSN(in settings.Database, password string) (string, error) {
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
