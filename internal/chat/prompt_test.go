package chat

import (
	"strings"
	"testing"

	"rag_project/internal/projects"
)

func projectWithPersona(persona projects.PersonaConfig) projects.Project {
	return projects.Project{
		Name:        "Solaria",
		Description: "Restaurant menu, prices and nutrition facts.",
		Persona:     persona,
	}
}

func TestSystemPromptUsesProjectAndPersona(t *testing.T) {
	prompt := SystemPrompt(projectWithPersona(projects.PersonaConfig{
		Name:            "Legal Assistant",
		Role:            "contract reviewer",
		Instructions:    "Always cite clause numbers.",
		Tone:            "Formal",
		AnswerLanguage:  "Indonesian",
		StrictKnowledge: true,
	}))

	checks := []string{
		"PROJECT CONTEXT",
		"Name: Solaria",
		"Restaurant menu, prices and nutrition facts.",
		"Legal Assistant",
		"contract reviewer",
		"Always cite clause numbers.",
		"formal tone",
		"Answer in Indonesian.",
		"OUTPUT FORMAT",
		"Separate paragraphs with a blank line",
		"Use only the knowledge context",
		"does not contain the answer",
	}
	for _, want := range checks {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestSystemPromptStrictVsGeneral(t *testing.T) {
	base := projects.PersonaConfig{Name: "Assistant", AnswerLanguage: "auto", StrictKnowledge: true}
	strict := SystemPrompt(projectWithPersona(base))
	generalPersona := base
	generalPersona.StrictKnowledge = false
	general := SystemPrompt(projectWithPersona(generalPersona))

	if !strings.Contains(strict, "Do not add facts") {
		t.Error("strict prompt must forbid outside knowledge")
	}
	if !strings.Contains(general, "you may answer from general knowledge") {
		t.Error("general prompt must allow outside knowledge")
	}
	if !strings.Contains(strict, "same language as the user's question") {
		t.Error("auto language must be represented")
	}
}

func TestUserPromptAndSources(t *testing.T) {
	hits := []projects.ChunkHit{
		{DocumentID: "d1", DocumentName: "policy.pdf", ChunkIndex: 2, PageNumber: 7, Content: "Clause text."},
		{DocumentID: "d1", DocumentName: "policy.pdf", ChunkIndex: 3, Content: "More text."},
		{DocumentID: "d2", DocumentName: "guide.md", ChunkIndex: 0, Content: "Guide text."},
	}
	prompt := UserPrompt("What is the limit?", hits)
	for _, want := range []string{"policy.pdf", "page 7", "Clause text.", "What is the limit?"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("user prompt missing %q", want)
		}
	}

	sources := Sources(hits)
	if len(sources) != 2 || sources[0].FileName != "policy.pdf" || sources[1].FileName != "guide.md" {
		t.Fatalf("unexpected sources: %+v", sources)
	}
}
