package mcp

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"promptline/internal/instance"
)

// CodexConfig returns a narrow, instance-specific MCP configuration fragment.
// The caller owns merging it into administrator configuration; this function
// never reads or overwrites CODEX_HOME configuration files.
func CodexConfig(executable string, in *instance.Instance) ([]byte, error) {
	if in == nil || executable == "" || !filepath.IsAbs(executable) {
		return nil, errors.New("MCP configuration requires an absolute executable and instance")
	}
	return json.MarshalIndent(map[string]any{"mcp_servers": map[string]any{
		"promptline-toolbox": map[string]any{
			"command": executable,
			"args":    []string{"toolbox", "serve", "--instance", in.Name(), "--cwd", in.WorkingDirectory(), "--state-root", in.StateRoot()},
		},
	}}, "", "  ")
}
