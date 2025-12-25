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
	for _, name := range names {
		data, err := promptFiles.ReadFile(name)
		if err != nil {
			return "", fmt.Errorf("failed to read system prompt file %q: %w", name, err)
		}
		content := string(data)
		builder.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			builder.WriteString("\n")
		}
	}

	return builder.String(), nil
}
