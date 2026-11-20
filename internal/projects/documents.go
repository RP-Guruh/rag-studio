package projects

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const documentColumns = `
	id, project_id, filename, COALESCE(mime_type, ''), COALESCE(size_bytes, 0),
	COALESCE(sha256, ''), COALESCE(storage_path, ''), status, COALESCE(error_message, ''),
	COALESCE(embedding_model, ''), COALESCE(embedding_dimension, 0), chunk_count,
	created_at, indexed_at
`

// ListDocuments returns every document stored for a project, newest first.
func (s *Store) ListDocuments(ctx context.Context, projectID string) ([]Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+documentColumns+`
		FROM documents
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	defer rows.Close()

	result := []Document{}
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}
	return result, nil
}

// GetDocument returns a single document belonging to a project.
func (s *Store) GetDocument(ctx context.Context, projectID, documentID string) (*Document, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+documentColumns+`
		FROM documents
		WHERE project_id = $1 AND id = $2
	`, projectID, documentID)

	doc, err := scanDocument(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// AddDocument persists a single uploaded file to disk and records its metadata.
// Uploading the same bytes twice is a no-op that returns the existing document.
func (s *Store) AddDocument(ctx context.Context, projectID string, input DocumentInput) (*Document, error) {
	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		return nil, errors.New("file name is required")
	}

	extension, err := docExtension(filename)
	if err != nil {
		return nil, err
	}

	data, err := base64.StdEncoding.DecodeString(stripDataURL(input.ContentBase64))
	if err != nil {
		return nil, fmt.Errorf("decode file %q: %w", filename, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("file %q is empty", filename)
	}
	if len(data) > MaxDocumentSize {
		return nil, fmt.Errorf("file %q exceeds the %d MB limit", filename, MaxDocumentSize>>20)
	}

	checksum := Checksum(data)
	if existing, err := s.getDocumentByChecksum(ctx, projectID, checksum); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	mimeType := strings.TrimSpace(input.MimeType)
	if mimeType == "" {
		mimeType = documentMimeTypes[extension]
	}

	storagePath, err := SaveDocumentFile(projectID, checksum, extension, data)
	if err != nil {
		return nil, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO documents (project_id, filename, mime_type, size_bytes, sha256, storage_path, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		ON CONFLICT (project_id, sha256) DO NOTHING
		RETURNING `+documentColumns+`
	`, projectID, filename, mimeType, len(data), checksum, storagePath)

	doc, err := scanDocument(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// A concurrent request stored the same file first. Drop the file we
			// just wrote unless it is the same path the existing row points at.
			existing, existingErr := s.getDocumentByChecksum(ctx, projectID, checksum)
			if existingErr != nil {
				return nil, existingErr
			}
			if existing != nil && existing.StoragePath != storagePath {
				_ = RemoveDocumentFile(storagePath)
			}
			return existing, nil
		}
		_ = RemoveDocumentFile(storagePath)
		return nil, fmt.Errorf("insert document metadata: %w", err)
	}
	return &doc, nil
}

// AddDocuments imports several files sequentially and returns all stored docs.
func (s *Store) AddDocuments(ctx context.Context, projectID string, inputs []DocumentInput) ([]Document, error) {
	result := make([]Document, 0, len(inputs))
	for _, input := range inputs {
		doc, err := s.AddDocument(ctx, projectID, input)
		if err != nil {
			return result, err
		}
		if doc != nil {
			result = append(result, *doc)
		}
	}
	return result, nil
}

// DeleteDocument removes a document row and its file from disk.
func (s *Store) DeleteDocument(ctx context.Context, projectID, documentID string) error {
	var storagePath string
	err := s.pool.QueryRow(ctx, `
		DELETE FROM documents
		WHERE project_id = $1 AND id = $2
		RETURNING COALESCE(storage_path, '')
	`, projectID, documentID).Scan(&storagePath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("delete document: %w", err)
	}
	if err := RemoveDocumentFile(storagePath); err != nil {
		return fmt.Errorf("document row deleted but local file could not be removed: %w", err)
	}
	return nil
}

func (s *Store) getDocumentByChecksum(ctx context.Context, projectID, checksum string) (*Document, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+documentColumns+`
		FROM documents
		WHERE project_id = $1 AND sha256 = $2
	`, projectID, checksum)

	doc, err := scanDocument(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query document by checksum: %w", err)
	}
	return &doc, nil
}

// EnsureDocumentStatusConstraint upgrades older databases whose status check
// predates the "chunked" state. It is a no-op once the constraint is current.
func (s *Store) EnsureDocumentStatusConstraint(ctx context.Context) error {
	var definition string
	err := s.pool.QueryRow(ctx, `
		SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conname = 'documents_status_check'
		  AND conrelid = 'documents'::regclass
	`).Scan(&definition)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("inspect documents status constraint: %w", err)
	}
	if err == nil && strings.Contains(definition, "'chunked'") {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin status constraint migration: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_status_check`); err != nil {
		return fmt.Errorf("drop documents status constraint: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		ALTER TABLE documents ADD CONSTRAINT documents_status_check
		CHECK (status IN ('pending','processing','chunked','indexed','failed'))
	`); err != nil {
		return fmt.Errorf("add documents status constraint: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit status constraint migration: %w", err)
	}
	return nil
}

// ListTrainableDocuments returns documents that still need chunking.
func (s *Store) ListTrainableDocuments(ctx context.Context, projectID string) ([]Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+documentColumns+`
		FROM documents
		WHERE project_id = $1 AND status IN ('pending', 'failed')
		ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("query trainable documents: %w", err)
	}
	defer rows.Close()

	result := []Document{}
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trainable documents: %w", err)
	}
	return result, nil
}

// ResetStaleProcessing moves documents left in "processing" by a previous run
// (for example after the app was closed mid-job) back to "pending".
func (s *Store) ResetStaleProcessing(ctx context.Context, projectID string) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE documents
		SET status = 'pending', error_message = NULL
		WHERE project_id = $1 AND status = 'processing'
	`, projectID)
	if err != nil {
		return 0, fmt.Errorf("reset stale processing documents: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ResetDocumentsForReindex resets all documents in the project back to "pending"
// and clears their chunk counts so training/indexing can run again completely.
func (s *Store) ResetDocumentsForReindex(ctx context.Context, projectID string) (int64, error) {
	_, _ = s.pool.Exec(ctx, `DELETE FROM chunks WHERE project_id = $1`, projectID)
	tag, err := s.pool.Exec(ctx, `
		UPDATE documents
		SET status = 'pending', chunk_count = 0, error_message = NULL
		WHERE project_id = $1
	`, projectID)
	if err != nil {
		return 0, fmt.Errorf("reset documents for reindex: %w", err)
	}
	return tag.RowsAffected(), nil
}

// MarkDocumentProcessing flags a document as actively being processed.
func (s *Store) MarkDocumentProcessing(ctx context.Context, projectID, documentID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE documents
		SET status = 'processing', error_message = NULL
		WHERE project_id = $1 AND id = $2
	`, projectID, documentID)
	if err != nil {
		return fmt.Errorf("mark document processing: %w", err)
	}
	return nil
}

// MarkDocumentChunked records a successful chunking run.
func (s *Store) MarkDocumentChunked(ctx context.Context, projectID, documentID string, chunkCount int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE documents
		SET status = 'chunked', chunk_count = $3, error_message = NULL
		WHERE project_id = $1 AND id = $2
	`, projectID, documentID, chunkCount)
	if err != nil {
		return fmt.Errorf("mark document chunked: %w", err)
	}
	return nil
}

// MarkDocumentFailed records a failed chunking run.
func (s *Store) MarkDocumentFailed(ctx context.Context, projectID, documentID, message string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE documents
		SET status = 'failed', error_message = $3
		WHERE project_id = $1 AND id = $2
	`, projectID, documentID, message)
	if err != nil {
		return fmt.Errorf("mark document failed: %w", err)
	}
	return nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanDocument(row rowScanner) (Document, error) {
	var doc Document
	err := row.Scan(
		&doc.ID, &doc.ProjectID, &doc.Filename, &doc.MimeType, &doc.SizeBytes,
		&doc.SHA256, &doc.StoragePath, &doc.Status, &doc.ErrorMessage,
		&doc.EmbeddingModel, &doc.EmbeddingDimension, &doc.ChunkCount,
		&doc.CreatedAt, &doc.IndexedAt,
	)
	if err != nil {
		return Document{}, err
	}
	return doc, nil
}

// stripDataURL removes an optional "data:<mime>;base64," prefix.
func stripDataURL(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, ";base64,"); idx >= 0 {
		return value[idx+len(";base64,"):]
	}
	return value
}
