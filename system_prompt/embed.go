package systemprompt

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.txt
var promptFiles embed.FS

// Load concatenates all embedded prompt files in lexical order.
func Load() (string, error) {
	entries, err := fs.ReadDir(promptFiles, ".")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded system prompt files: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		names = append(names, entry.Name())
	}

	if len(names) == 0 {
		return "", fmt.Errorf("no system prompt files found in embedded set")
	}

	sort.Strings(names)

	var builder strings.Builder
	for idx, name := range names {
		data, err := promptFiles.ReadFile(name)
		if err != nil {
			return "", fmt.Errorf("failed to read system prompt file %q: %w", name, err)
		}
		builder.WriteString(string(data))
		if !strings.HasSuffix(builder.String(), "\n") {
			builder.WriteString("\n")
		}
		if idx < len(names)-1 {
			// Separate prompts with a newline for clarity.
			builder.WriteString("\n")
		}
	}

	return builder.String(), nil
}
