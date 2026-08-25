package runtime

import (
	"strings"
	"testing"
)

func TestEmbeddedInitPromptContainsToolboxGuidance(t *testing.T) {
	prompt := initPrompt()
	for _, expected := range []string{"# Promptline runtime instructions", "promptline-toolbox", "MCP"} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("embedded init prompt is missing %q: %q", expected, prompt)
		}
	}
}
