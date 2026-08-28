package runtime

import (
	_ "embed"
	"fmt"
	"strings"

	"promptline/plugins/promptline/skills"
)

//go:embed init-prompt.md
var embeddedInitPrompt string

func initPrompt() (string, error) {
	catalog, err := skills.EmbeddedCatalog()
	if err != nil {
		return "", fmt.Errorf("load embedded skill discovery metadata: %w", err)
	}
	return strings.TrimSpace(embeddedInitPrompt) + "\n\n" + catalog.BootstrapInstructions(), nil
}
