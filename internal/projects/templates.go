package projects

import (
	"context"
	"fmt"
)

// PromptTemplate is a reusable persona instruction block.
type PromptTemplate struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Instructions string `json:"instructions"`
}

// ListPromptTemplates returns saved templates plus built-in defaults.
func (s *Store) ListPromptTemplates(ctx context.Context) ([]PromptTemplate, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, name, instructions FROM prompt_templates ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list prompt templates: %w", err)
	}
	defer rows.Close()
	result := []PromptTemplate{}
	for rows.Next() {
		var template PromptTemplate
		if err := rows.Scan(&template.ID, &template.Name, &template.Instructions); err != nil {
			return nil, err
		}
		result = append(result, template)
	}
	return result, nil
}

// SavePromptTemplate upserts a template by name.
func (s *Store) SavePromptTemplate(ctx context.Context, name, instructions string) (*PromptTemplate, error) {
	var template PromptTemplate
	err := s.pool.QueryRow(ctx, `
		INSERT INTO prompt_templates (name, instructions) VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET instructions = excluded.instructions
		RETURNING id::text, name, instructions
	`, name, instructions).Scan(&template.ID, &template.Name, &template.Instructions)
	if err != nil {
		return nil, fmt.Errorf("save prompt template: %w", err)
	}
	return &template, nil
}
