package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// GetRetrievalConfig returns the project's retrieval settings, or defaults.
func (s *Store) GetRetrievalConfig(ctx context.Context, projectID string) (RetrievalConfig, error) {
	var cfg RetrievalConfig
	err := s.pool.QueryRow(ctx, `
		SELECT top_k, min_score, hybrid, rerank, guardrails, hybrid_weight, rerank_weight
		FROM project_retrieval WHERE project_id = $1::uuid
	`, projectID).Scan(&cfg.TopK, &cfg.MinScore, &cfg.Hybrid, &cfg.Rerank, &cfg.Guardrails, &cfg.HybridWeight, &cfg.RerankWeight)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RetrievalConfig{TopK: 8}, nil
		}
		return cfg, fmt.Errorf("load retrieval config: %w", err)
	}
	return cfg, nil
}

// UpdateRetrievalConfig upserts the project's retrieval settings.
func (s *Store) UpdateRetrievalConfig(ctx context.Context, projectID string, cfg RetrievalConfig) error {
	if cfg.TopK < 1 || cfg.TopK > 20 {
		cfg.TopK = 8
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO project_retrieval (project_id, top_k, min_score, hybrid, rerank, guardrails, hybrid_weight, rerank_weight)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (project_id) DO UPDATE SET
			top_k = excluded.top_k,
			min_score = excluded.min_score,
			hybrid = excluded.hybrid,
			rerank = excluded.rerank,
			guardrails = excluded.guardrails,
			hybrid_weight = excluded.hybrid_weight,
			rerank_weight = excluded.rerank_weight
	`, projectID, cfg.TopK, cfg.MinScore, cfg.Hybrid, cfg.Rerank, cfg.Guardrails, cfg.HybridWeight, cfg.RerankWeight)
	if err != nil {
		return fmt.Errorf("save retrieval config: %w", err)
	}
	return nil
}

// CreateEvalSet adds an evaluation set to a project.
func (s *Store) CreateEvalSet(ctx context.Context, projectID, name string) (*EvalSet, error) {
	var set EvalSet
	err := s.pool.QueryRow(ctx, `
		INSERT INTO eval_sets (project_id, name) VALUES ($1::uuid, $2)
		RETURNING id::text, project_id::text, name
	`, projectID, name).Scan(&set.ID, &set.ProjectID, &set.Name)
	if err != nil {
		return nil, fmt.Errorf("create eval set: %w", err)
	}
	return &set, nil
}

// ListEvalSets returns all evaluation sets of a project.
func (s *Store) ListEvalSets(ctx context.Context, projectID string) ([]EvalSet, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, project_id::text, name FROM eval_sets
		WHERE project_id = $1::uuid ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list eval sets: %w", err)
	}
	defer rows.Close()
	result := []EvalSet{}
	for rows.Next() {
		var set EvalSet
		if err := rows.Scan(&set.ID, &set.ProjectID, &set.Name); err != nil {
			return nil, err
		}
		result = append(result, set)
	}
	return result, nil
}

// GetEvalSet returns one evaluation set.
func (s *Store) GetEvalSet(ctx context.Context, setID string) (*EvalSet, error) {
	var set EvalSet
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, project_id::text, name FROM eval_sets WHERE id = $1::uuid
	`, setID).Scan(&set.ID, &set.ProjectID, &set.Name)
	if err != nil {
		return nil, fmt.Errorf("load eval set: %w", err)
	}
	return &set, nil
}

// AddEvalCase appends a case to an evaluation set.
func (s *Store) AddEvalCase(ctx context.Context, setID, question, expectedAnswer, expectedSource string) (*EvalCase, error) {
	var evalCase EvalCase
	err := s.pool.QueryRow(ctx, `
		INSERT INTO eval_cases (set_id, question, expected_answer, expected_source_name)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, set_id::text, question, expected_answer, expected_source_name
	`, setID, question, expectedAnswer, expectedSource).Scan(
		&evalCase.ID, &evalCase.SetID, &evalCase.Question, &evalCase.ExpectedAnswer, &evalCase.ExpectedSourceName,
	)
	if err != nil {
		return nil, fmt.Errorf("add eval case: %w", err)
	}
	return &evalCase, nil
}

// ListEvalCases returns the cases of an evaluation set.
func (s *Store) ListEvalCases(ctx context.Context, setID string) ([]EvalCase, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, set_id::text, question, expected_answer, expected_source_name
		FROM eval_cases WHERE set_id = $1::uuid ORDER BY created_at
	`, setID)
	if err != nil {
		return nil, fmt.Errorf("list eval cases: %w", err)
	}
	defer rows.Close()
	result := []EvalCase{}
	for rows.Next() {
		var evalCase EvalCase
		if err := rows.Scan(&evalCase.ID, &evalCase.SetID, &evalCase.Question, &evalCase.ExpectedAnswer, &evalCase.ExpectedSourceName); err != nil {
			return nil, err
		}
		result = append(result, evalCase)
	}
	return result, nil
}
