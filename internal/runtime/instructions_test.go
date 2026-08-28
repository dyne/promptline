package runtime

import (
	"strings"
	"testing"
)

func TestEmbeddedInitPromptContainsToolboxGuidance(t *testing.T) {
	prompt, err := initPrompt()
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# Promptline runtime instructions", "promptline-toolbox", "MCP"} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("embedded init prompt is missing %q: %q", expected, prompt)
		}
	}
}

func TestEmbeddedInitPromptBootstrapsAuthoritativeSkill(t *testing.T) {
	prompt, err := initPrompt()
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"bash-defensive-patterns",
		"bash-linux",
		"debian-sysadmin",
		"security-ownership-map",
		"security-threat-model",
		"skill://<name>/SKILL.md",
		"skill-bundle://promptline/LICENSE.txt",
		"MCP-served content as authoritative",
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("embedded init prompt is missing %q: %q", expected, prompt)
		}
	}
}
