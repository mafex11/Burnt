package engine

import "strings"

// Tool is the coding agent a model belongs to.
type Tool string

const (
	ToolClaude Tool = "claude"
	ToolCodex  Tool = "codex"
)

// codexPrefixes are the OpenAI model families Codex uses.
var codexPrefixes = []string{"gpt-", "o1", "o3", "codex"}

// ClassifyModel maps a model name to its originating tool by name prefix.
// Codex (OpenAI) families: gpt-*, o1*, o3*, codex*. Everything else is treated as
// Claude — a conservative default, since Anthropic is the primary tool.
func ClassifyModel(name string) Tool {
	lower := strings.ToLower(name)
	for _, p := range codexPrefixes {
		if strings.HasPrefix(lower, p) {
			return ToolCodex
		}
	}
	return ToolClaude
}
