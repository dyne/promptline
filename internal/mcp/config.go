package mcp

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"promptline/internal/instance"
	"promptline/internal/tools"
)

var readOnlyTools = []string{
	"base64", "basename", "cat", "cmp", "comm", "date", "df", "dirname",
	"du", "echo", "find", "free", "get_current_datetime", "grep", "head",
	"hexdump", "hostname", "id", "ls", "md5sum", "more", "pidof", "printenv",
	"ps", "pwd", "read_file", "readlink", "realpath", "seq", "shasum", "sort",
	"strings", "tail", "tr", "tty", "uname", "uniq", "uptime", "wc", "which",
}

// ReadOnlyToolPolicy allows observation-only tools in the noninteractive MCP
// subprocess. Mutating tools remain ask-by-default and therefore fail closed.
func ReadOnlyToolPolicy() tools.Policy {
	return tools.PolicyFromLists(readOnlyTools, nil, nil)
}

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
