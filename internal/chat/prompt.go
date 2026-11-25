package chat

import (
	"fmt"
	"strings"

	"rag_project/internal/projects"
)

// SystemPrompt builds the system message from a project's identity and persona.
// The project name and description give the model context about the knowledge
// base it is answering from; the persona fields control identity, instructions,
// tone, language and grounding.
func SystemPrompt(project projects.Project) string {
	var b strings.Builder

	b.WriteString("PROJECT CONTEXT\n")
	fmt.Fprintf(&b, "Name: %s\n", strings.TrimSpace(project.Name))
	if description := strings.TrimSpace(project.Description); description != "" {
		fmt.Fprintf(&b, "Description: %s\n", description)
	}
	b.WriteString("Use this as background for understanding the user's question.\n\n")

	persona := project.Persona
	name := strings.TrimSpace(persona.Name)
	if name == "" {
		name = "Assistant"
	}
	b.WriteString("IDENTITY\n")
	fmt.Fprintf(&b, "You are %s", name)
	if role := strings.TrimSpace(persona.Role); role != "" {
		fmt.Fprintf(&b, ", %s", role)
	}
	b.WriteString(".\n\n")

	b.WriteString("INSTRUCTIONS\n")
	if instructions := strings.TrimSpace(persona.Instructions); instructions != "" {
		b.WriteString(instructions)
		b.WriteString("\n\n")
	} else {
		b.WriteString("Answer the user's question clearly and directly.\n\n")
	}

	b.WriteString("TONE\n")
	tone := strings.TrimSpace(persona.Tone)
	if tone == "" {
		tone = "neutral"
	}
	fmt.Fprintf(&b, "Use a %s tone.\n\n", strings.ToLower(tone))

	b.WriteString("LANGUAGE\n")
	language := strings.TrimSpace(persona.AnswerLanguage)
	if language == "" || language == "auto" {
		b.WriteString("Answer in the same language as the user's question.\n\n")
	} else {
		fmt.Fprintf(&b, "Answer in %s.\n\n", language)
	}

	b.WriteString("OUTPUT FORMAT\n")
	b.WriteString("Reply in Markdown. Separate paragraphs with a blank line. Use short bullet points or numbered lists for multiple items. Use a short heading when it helps. Write complete sentences and do not answer with one dense paragraph.\n\n")

	b.WriteString("GROUNDING RULES\n")
	if persona.StrictKnowledge {
		b.WriteString("Use only the knowledge context provided below. Do not add facts, opinions, or details from outside that context. ")
		b.WriteString("If the answer is not present in the context, reply that the knowledge base does not contain the answer. ")
		b.WriteString("Never invent a source.\n")
	} else {
		b.WriteString("Use the knowledge context when it is relevant. If it does not cover the question, you may answer from general knowledge, but say when you are doing so.\n")
	}
	return b.String()
}

// UserPrompt combines the retrieved evidence with the question. Chunks are
// numbered so the model can associate an answer with a specific document.
func UserPrompt(question string, hits []projects.ChunkHit) string {
	var b strings.Builder
	b.WriteString("KNOWLEDGE CONTEXT\n")
	if len(hits) == 0 {
		b.WriteString("No relevant chunks were found.\n")
	} else {
		for _, hit := range hits {
			location := ""
			if hit.PageNumber > 0 {
				location = fmt.Sprintf(", page %d", hit.PageNumber)
			}
			fmt.Fprintf(&b, "[%s, chunk %d%s]\n%s\n\n", hit.DocumentName, hit.ChunkIndex, location, hit.Content)
		}
	}
	fmt.Fprintf(&b, "QUESTION\n%s", strings.TrimSpace(question))
	return b.String()
}

// Sources returns each cited document once, in retrieval order.
func Sources(hits []projects.ChunkHit) []projects.ChatSource {
	result := []projects.ChatSource{}
	seen := map[string]bool{}
	for _, hit := range hits {
		if seen[hit.DocumentID] {
			continue
		}
		seen[hit.DocumentID] = true
		result = append(result, projects.ChatSource{DocumentID: hit.DocumentID, FileName: hit.DocumentName})
	}
	return result
}
