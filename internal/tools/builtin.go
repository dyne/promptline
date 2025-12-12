// Copyright (C) 2025 Dyne.org foundation
// designed, written and maintained by Denis Roio <jaromil@dyne.org>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/criyle/go-sandbox/runner"
)

// registerBuiltInTools registers all built-in tools to the registry
func registerBuiltInTools(r *Registry) {
	r.RegisterTool(&Tool{
		Name:        "get_current_datetime",
		Description: "Get the current date and time in ISO 8601 format",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Executor: getCurrentDatetime,
	})

	r.RegisterTool(&Tool{
		Name:        "execute_shell_command",
		Description: "Execute a shell command and return its output",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute",
				},
			},
			"required": []string{"command"},
		},
		Executor: executeShellCommand,
	})

	r.RegisterTool(&Tool{
		Name:        "read_file",
		Description: "Read the contents of a file",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"path"},
		},
		Executor: readFile,
	})

	r.RegisterTool(&Tool{
		Name:        "write_file",
		Description: "Write content to a file",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to write",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
		Executor: writeFile,
	})

	r.RegisterTool(&Tool{
		Name:        "ls",
		Description: "List directory contents with detailed information. Can recursively traverse directories.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory path to list (default: current directory)",
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to list recursively (default: false)",
				},
				"show_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to show hidden files (default: false)",
				},
			},
		},
		Executor: listDirectory,
	})
}

// Security constants for validation
const (
	maxCommandLength = 10000
	maxPathLength    = 4096
	commandTimeout   = 30 * time.Second
)

// Dangerous path patterns that should be blocked
var dangerousPaths = []string{
	"/etc/", "/sys/", "/proc/", "/dev/",
	"/boot/", "/root/", "/var/run/", "/var/lib/",
}

// Command injection patterns to block
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[;&|]\s*rm\s`),         // rm after separator
	regexp.MustCompile(`[;&|]\s*dd\s`),         // dd after separator
	regexp.MustCompile(`>\s*/dev/`),            // redirect to /dev
	regexp.MustCompile(`/etc/(passwd|shadow)`), // system files
	regexp.MustCompile(`curl.*\|\s*(sh|bash)`), // curl pipe to shell
	regexp.MustCompile(`wget.*\|\s*(sh|bash)`), // wget pipe to shell
}

// validateCommand checks for dangerous patterns and length limits
func validateCommand(command string) error {
	if len(command) > maxCommandLength {
		return fmt.Errorf("command exceeds maximum length of %d characters", maxCommandLength)
	}

	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("command cannot be empty")
	}

	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains potentially dangerous pattern: %s", pattern.String())
		}
	}

	return nil
}

// validatePath checks if a path is safe to access
func validatePath(path string) error {
	if len(path) > maxPathLength {
		return fmt.Errorf("path exceeds maximum length of %d characters", maxPathLength)
	}

	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Clean and get absolute path
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid path: %v", err)
	}

	// Check against dangerous paths
	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(absPath, dangerous) {
			return fmt.Errorf("access to %s is restricted for security", dangerous)
		}
	}

	return nil
}

// Tool implementations

func getCurrentDatetime(args map[string]interface{}) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}

func executeShellCommand(args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'command' parameter")
	}

	// Validate command for security
	if err := validateCommand(command); err != nil {
		return "", fmt.Errorf("command validation failed: %v", err)
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	if sandboxRunner != nil {
		outBuf := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}
		result, err := sandboxRunner.ExecInSandbox(ctx, "sh", []string{"-c", command}, nil, outBuf, errBuf)

		output := outBuf.String() + errBuf.String()

		if ctx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("command timed out after %v", commandTimeout)
		}

		// If sandbox unavailable, fall back.
		if err != nil {
			return executeShellCommandHost(ctx, command)
		}

		if result.Status != runner.StatusNormal || result.ExitStatus != 0 {
			return output, fmt.Errorf("command failed: status=%d exit=%d", result.Status, result.ExitStatus)
		}
		return output, nil
	}

	return executeShellCommandHost(ctx, command)
}

func executeShellCommandHost(ctx context.Context, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out after %v", commandTimeout)
	}

	if err != nil {
		return string(output), fmt.Errorf("command failed: %v", err)
	}

	return string(output), nil
}

func readFile(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' parameter")
	}

	resolved, err := validatePathWithinWorkdir(path)
	if err != nil {
		return "", err
	}

	// Use os.ReadFile instead of exec for better security
	content, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	return string(content), nil
}

func writeFile(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'path' parameter")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'content' parameter")
	}

	resolved, err := validatePathWithinWorkdir(path)
	if err != nil {
		return "", err
	}

	// Use os.WriteFile instead of exec for better security
	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), resolved), nil
}

func listDirectory(args map[string]interface{}) (string, error) {
	path := getPathArg(args)
	if err := validateDirectoryPath(path); err != nil {
		return "", err
	}

	recursive := getBoolArg(args, "recursive")
	showHidden := getBoolArg(args, "show_hidden")

	var result strings.Builder
	var err error

	if recursive {
		err = walkDirectory(path, showHidden, &result)
	} else {
		err = listDirectoryNonRecursive(path, showHidden, &result)
	}

	if err != nil {
		return "", err
	}

	if result.Len() == 0 {
		return "Directory is empty", nil
	}

	return result.String(), nil
}

func getPathArg(args map[string]interface{}) string {
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "."
	}
	return path
}

func getBoolArg(args map[string]interface{}, key string) bool {
	val, ok := args[key].(bool)
	return ok && val
}

func validateDirectoryPath(path string) error {
	if path != "." {
		if _, err := validatePathWithinWorkdir(path); err != nil {
			return fmt.Errorf("path validation failed: %v", err)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path not found: %v", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path '%s' is not a directory", path)
	}

	return nil
}

func validatePathWithinWorkdir(path string) (string, error) {
	if len(path) > maxPathLength {
		return "", fmt.Errorf("path exceeds maximum length of %d characters", maxPathLength)
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}

	// Prevent access to masked/dangerous paths even if under workdir.
	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(absPath, dangerous) {
			return "", fmt.Errorf("access to %s is restricted for security", dangerous)
		}
	}

	if sandboxWorkdir == "" {
		return absPath, nil
	}
	workdirAbs, err := filepath.Abs(filepath.Clean(sandboxWorkdir))
	if err != nil {
		return "", fmt.Errorf("invalid workdir: %v", err)
	}
	if !strings.HasPrefix(absPath, workdirAbs+string(filepath.Separator)) && absPath != workdirAbs {
		return "", fmt.Errorf("access denied: path must stay within workdir")
	}
	return absPath, nil
}

func walkDirectory(path string, showHidden bool, result *strings.Builder) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if shouldSkipHidden(filePath, info, path, showHidden) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		formatEntry(result, filePath, info, path)
		return nil
	})
}

func listDirectoryNonRecursive(path string, showHidden bool, result *strings.Builder) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err)
	}

	for _, entry := range entries {
		if !showHidden && isHidden(entry.Name()) {
			continue
		}

		entryPath := filepath.Join(path, entry.Name())
		info, err := entry.Info()
		if err != nil {
			result.WriteString(fmt.Sprintf("%-40s <error reading info>\n", entry.Name()))
			continue
		}

		formatEntry(result, entryPath, info, path)
	}

	return nil
}

func shouldSkipHidden(filePath string, info os.FileInfo, basePath string, showHidden bool) bool {
	return !showHidden && isHidden(filepath.Base(filePath)) && filePath != basePath
}

func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// formatEntry formats a single directory entry for output
func formatEntry(result *strings.Builder, filePath string, info os.FileInfo, basePath string) {
	// Get relative path for cleaner output
	relPath, err := filepath.Rel(basePath, filePath)
	if err != nil {
		relPath = filePath
	}

	// Determine type
	typeStr := "-"
	if info.IsDir() {
		typeStr = "d"
	} else if info.Mode()&os.ModeSymlink != 0 {
		typeStr = "l"
	}

	// Format permissions
	perms := info.Mode().Perm().String()

	// Format size (human-readable)
	size := formatSize(info.Size())

	// Format modified time
	modTime := info.ModTime().Format("2006-01-02 15:04:05")

	// Write formatted line
	result.WriteString(fmt.Sprintf("%s %s %8s %s %s\n",
		typeStr, perms, size, modTime, relPath))
}

// formatSize converts bytes to human-readable format
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	sizes := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f%s", float64(bytes)/float64(div), sizes[exp])
}
