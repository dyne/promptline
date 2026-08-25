package mcp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

// CodexConfig returns the complete Promptline-owned Codex configuration for an
// instance. CODEX_HOME is private to that instance, so no user configuration
// is merged into this file.
func CodexConfig(executable string, in *instance.Instance) ([]byte, error) {
	if in == nil || executable == "" || !filepath.IsAbs(executable) {
		return nil, errors.New("MCP configuration requires an absolute executable and instance")
	}
	args := []string{"mcp-server", "--cwd", in.WorkingDirectory()}
	quotedArgs := make([]string, len(args))
	for index, arg := range args {
		quotedArgs[index] = strconv.Quote(arg)
	}
	config := fmt.Sprintf("[mcp_servers.promptline-toolbox]\ncommand = %s\nargs = [%s]\nenabled = true\nrequired = true\ndefault_tools_approval_mode = \"approve\"\n",
		strconv.Quote(executable), strings.Join(quotedArgs, ", "))
	return []byte(config), nil
}

// InstallCodexConfig atomically installs the instance-owned Codex config with
// private permissions before the app-server process starts.
func InstallCodexConfig(executable string, in *instance.Instance) error {
	data, err := CodexConfig(executable, in)
	if err != nil {
		return err
	}
	directory := in.CodexHome()
	temporary, err := os.CreateTemp(directory, ".config-*.toml")
	if err != nil {
		return fmt.Errorf("create Codex config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure Codex config: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write Codex config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync Codex config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close Codex config: %w", err)
	}
	if err := os.Rename(temporaryPath, filepath.Join(directory, "config.toml")); err != nil {
		return fmt.Errorf("install Codex config: %w", err)
	}
	dir, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open CODEX_HOME: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync CODEX_HOME: %w", err)
	}
	return nil
}
