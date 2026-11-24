package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ListChunks returns every stored chunk of a project for export.
func (s *Store) ListChunks(ctx context.Context, projectID string) ([]Chunk, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT document_id::text, project_id::text, chunk_index, content,
		       COALESCE(token_count, 0), COALESCE(page_number, 0),
		       COALESCE(metadata::text, '{}'), embedding::text
		FROM chunks WHERE project_id = $1::uuid
		ORDER BY document_id, chunk_index
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list chunks: %w", err)
	}
	defer rows.Close()
	result := []Chunk{}
	for rows.Next() {
		var chunk Chunk
		var metadataText, embeddingText string
		if err := rows.Scan(&chunk.DocumentID, &chunk.ProjectID, &chunk.ChunkIndex, &chunk.Content,
			&chunk.TokenCount, &chunk.PageNumber, &metadataText, &embeddingText); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		if err := json.Unmarshal([]byte(metadataText), &chunk.Metadata); err != nil {
			chunk.Metadata = map[string]any{}
		}
		chunk.Embedding = parseVectorLiteral(embeddingText)
		result = append(result, chunk)
	}
	return result, nil
}

func parseVectorLiteral(value string) []float32 {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.TrimSuffix(value, "]"), "[")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]float32, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
		if err != nil {
			return nil
		}
		out = append(out, float32(number))
	}
	return out
}

// VectorLiteral renders a vector as a pgvector literal. Callers cast it to
// vector(N) in SQL, so the dimension is checked by PostgreSQL as well.
func VectorLiteral(vector []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, value := range vector {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// ReplaceDocumentChunks atomically swaps the stored chunks of one document.
// The embedding column is NOT NULL, so all vectors must be supplied up front.
func (s *Store) ReplaceDocumentChunks(ctx context.Context, projectID, documentID string, chunks []Chunk) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin chunk replacement: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM chunks WHERE project_id = $1 AND document_id = $2`, projectID, documentID); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}

	for i, chunk := range chunks {
		if len(chunk.Embedding) == 0 {
			return fmt.Errorf("chunk %d has no embedding", i)
		}
		metadata := chunk.Metadata
		if metadata == nil {
			metadata = map[string]any{}
		}
		encoded, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("encode chunk %d metadata: %w", i, err)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO chunks
				(document_id, project_id, chunk_index, content, token_count, page_number, metadata, embedding)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::vector)
		`, chunk.DocumentID, chunk.ProjectID, chunk.ChunkIndex, chunk.Content,
			chunk.TokenCount, chunk.PageNumber, string(encoded), VectorLiteral(chunk.Embedding))
		if err != nil {
			return fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit chunk replacement: %w", err)
	}
	return nil
}

// MarkDocumentIndexed records a fully indexed document.
func (s *Store) MarkDocumentIndexed(ctx context.Context, projectID, documentID, model string, dimension, chunkCount int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE documents
		SET status = 'indexed',
		    indexed_at = now(),
		    embedding_model = $3,
		    embedding_dimension = $4,
		    chunk_count = $5,
		    error_message = NULL
		WHERE project_id = $1 AND id = $2
	`, projectID, documentID, model, dimension, chunkCount)
	if err != nil {
		return fmt.Errorf("mark document indexed: %w", err)
	}
	return nil
}

// SearchChunksWithConfig applies the project's stored top-K and minimum score.
func (s *Store) SearchChunksWithConfig(ctx context.Context, projectID string, query []float32, cfg RetrievalConfig) ([]ChunkHit, error) {
	return s.SearchChunksForQuery(ctx, projectID, "", query, cfg)
}

// SearchChunksForQuery adds optional keyword search and reranking on top of
// vector retrieval. Keyword scores come from PostgreSQL full-text search.
func (s *Store) SearchChunksForQuery(ctx context.Context, projectID, queryText string, query []float32, cfg RetrievalConfig) ([]ChunkHit, error) {
	poolSize := cfg.TopK
	if cfg.Hybrid || cfg.Rerank {
		poolSize *= 3
	}
	vectorHits, err := s.SearchChunks(ctx, projectID, query, poolSize)
	if err != nil {
		return nil, err
	}
	filtered := make([]ChunkHit, 0, len(vectorHits))
	for _, hit := range vectorHits {
		if hit.Score >= cfg.MinScore {
			filtered = append(filtered, hit)
		}
	}
	if !cfg.Hybrid || strings.TrimSpace(queryText) == "" {
		return limitHits(filtered, cfg.TopK), nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.document_id, d.filename, c.chunk_index, c.content,
		       COALESCE(c.page_number, 0),
		       ts_rank(to_tsvector('simple', c.content), plainto_tsquery('simple', $2)) AS kw,
		       COALESCE(c.metadata::text, '{}')
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.project_id = $1
		  AND to_tsvector('simple', c.content) @@ plainto_tsquery('simple', $2)
		ORDER BY kw DESC
		LIMIT $3
	`, projectID, queryText, poolSize)
	if err != nil {
		return nil, fmt.Errorf("keyword search: %w", err)
	}
	defer rows.Close()

	combined := map[int64]ChunkHit{}
	for _, hit := range filtered {
		combined[hit.ChunkID] = hit
	}
	for rows.Next() {
		var hit ChunkHit
		var keywordScore float64
		var metadataText string
		if err := rows.Scan(&hit.ChunkID, &hit.DocumentID, &hit.DocumentName, &hit.ChunkIndex,
			&hit.Content, &hit.PageNumber, &keywordScore, &metadataText); err != nil {
			return nil, fmt.Errorf("scan keyword hit: %w", err)
		}
		if err := json.Unmarshal([]byte(metadataText), &hit.Metadata); err != nil {
			hit.Metadata = map[string]any{}
		}
		hybridWeight := cfg.HybridWeight
		if hybridWeight <= 0 {
			hybridWeight = 0.25
		}
		rerankWeight := cfg.RerankWeight
		if rerankWeight <= 0 {
			rerankWeight = 0.25
		}
		if existing, ok := combined[hit.ChunkID]; ok {
			if cfg.Rerank {
				existing.Score += keywordScore * rerankWeight
			}
			combined[hit.ChunkID] = existing
		} else {
			hit.Score = keywordScore * hybridWeight
			combined[hit.ChunkID] = hit
		}
	}
	hits := make([]ChunkHit, 0, len(combined))
	for _, hit := range combined {
		hits = append(hits, hit)
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	return limitHits(hits, cfg.TopK), nil
}

func limitHits(hits []ChunkHit, limit int) []ChunkHit {
	if limit < 1 {
		limit = 8
	}
	if len(hits) > limit {
		return hits[:limit]
	}
	return hits
}

// SearchChunks finds the chunks of one project closest to the query vector,
// ordered by cosine distance. The project filter keeps workspaces isolated.
func (s *Store) SearchChunks(ctx context.Context, projectID string, query []float32, limit int) ([]ChunkHit, error) {
	if len(query) == 0 {
		return nil, errors.New("query embedding is empty")
	}
	if limit < 1 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}

	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.document_id, d.filename, c.chunk_index, c.content,
		       COALESCE(c.page_number, 0),
		       1 - (c.embedding <=> $2::vector) AS score,
		       COALESCE(c.metadata::text, '{}')
		FROM chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE c.project_id = $1
		ORDER BY c.embedding <=> $2::vector
		LIMIT $3
	`, projectID, VectorLiteral(query), limit)
	if err != nil {
		return nil, fmt.Errorf("search chunks: %w", err)
	}
	defer rows.Close()

	result := []ChunkHit{}
	for rows.Next() {
		var hit ChunkHit
		var metadataText string
		if err := rows.Scan(
			&hit.ChunkID, &hit.DocumentID, &hit.DocumentName, &hit.ChunkIndex,
			&hit.Content, &hit.PageNumber, &hit.Score, &metadataText,
		); err != nil {
			return nil, fmt.Errorf("scan chunk hit: %w", err)
		}
		if err := json.Unmarshal([]byte(metadataText), &hit.Metadata); err != nil {
			hit.Metadata = map[string]any{}
		}
		result = append(result, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chunk hits: %w", err)
	}
	return result, nil
}
