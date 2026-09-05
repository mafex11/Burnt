package engine

import "testing"

func TestClassifyModelClaude(t *testing.T) {
	for _, name := range []string{"claude-opus-4-8", "claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001"} {
		if got := ClassifyModel(name); got != ToolClaude {
			t.Errorf("ClassifyModel(%q) = %q, want claude", name, got)
		}
	}
}

func TestClassifyModelCodex(t *testing.T) {
	for _, name := range []string{"gpt-5.4", "gpt-5.2-codex", "o3", "o1-mini", "codex-mini", "GPT-5.4"} {
		if got := ClassifyModel(name); got != ToolCodex {
			t.Errorf("ClassifyModel(%q) = %q, want codex", name, got)
		}
	}
}

// Conservative default: unknown model names are treated as Claude, since Anthropic is
// the primary tool.
func TestClassifyModelUnknownDefaultsToClaude(t *testing.T) {
	if got := ClassifyModel("mystery-model-7"); got != ToolClaude {
		t.Errorf("ClassifyModel(mystery-model-7) = %q, want claude", got)
	}
}
