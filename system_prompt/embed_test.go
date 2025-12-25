package systemprompt

import (
	"os"
	"sort"
	"strings"
	"testing"
)

func TestLoadConcatenatesPromptFiles(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read system_prompt dir: %v", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		names = append(names, entry.Name())
	}

	if len(names) == 0 {
		t.Fatal("expected at least one .txt file in system_prompt")
	}

	sort.Strings(names)

	var expected strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		content := string(data)
		expected.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			expected.WriteString("\n")
		}
	}

	prompt, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if prompt != expected.String() {
		t.Fatalf("Load() output mismatch")
	}
}
