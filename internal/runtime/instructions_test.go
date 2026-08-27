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

func TestEmbeddedInitPromptBootstrapsAuthoritativeSkill(t *testing.T) {
	prompt := initPrompt()
	for _, expected := range []string{
		"debian-sysadmin",
		"skill://debian-sysadmin/SKILL.md",
		"skill://debian-sysadmin/...",
		"MCP\nresource content is authoritative",
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("embedded init prompt is missing %q: %q", expected, prompt)
		}
	}
}
