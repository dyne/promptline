package runtime

import (
	_ "embed"
	"strings"
)

//go:embed init-prompt.md
var embeddedInitPrompt string

func initPrompt() string {
	return strings.TrimSpace(embeddedInitPrompt)
}
