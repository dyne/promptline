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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"promptline/internal/paths"
)

// registerBuiltInTools registers all built-in tools to the registry
func registerBuiltInTools(r *Registry) {
	register := func(tool Tool) {
		if err := r.RegisterTool(tool); err != nil {
			panic(err)
		}
	}

	register(&ToolDefinition{
		NameValue:        "get_current_datetime",
		DescriptionValue: "Get the current date and time in ISO 8601 format",
		ParametersValue: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		ExecuteFunc:  getCurrentDatetime,
		VersionValue: builtinToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "execute_shell_command",
		DescriptionValue: "Execute a shell command and return its output (do not use for writing files; use write_file)",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute",
				},
			},
			"required": []string{"command"},
		},
		ExecuteFunc:  executeShellCommand,
		ValidateFunc: RequireStringArg("command", "missing or invalid 'command' parameter (use write_file for file writes)"),
		VersionValue: builtinToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "read_file",
		DescriptionValue: "Read the contents of a file",
		ParametersValue: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"path"},
		},
		ExecuteFunc:  readFile,
		ValidateFunc: RequireNonEmptyArg("path", "missing or invalid 'path' parameter"),
		VersionValue: builtinToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "write_file",
		DescriptionValue: "Create or overwrite a text file (preferred for file writes)",
		ParametersValue: map[string]interface{}{
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
		ExecuteFunc: writeFile,
		ValidateFunc: ChainValidation(
			RequireStringArg("path", "missing or invalid 'path' or 'content' parameter"),
			RequireStringArg("content", "missing or invalid 'path' or 'content' parameter"),
		),
		VersionValue: builtinToolVersion,
	})

	register(&ToolDefinition{
		NameValue:        "ls",
		DescriptionValue: "List directory contents with detailed information. Can recursively traverse directories.",
		ParametersValue: map[string]interface{}{
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
		ExecuteFunc:  listDirectory,
		VersionValue: builtinToolVersion,
	})
}

const builtinToolVersion = "1.0.0"

// Security constants for validation
const (
	maxCommandLength = 10000
	maxPathLength    = 4096
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
	if err := paths.ValidatePathString(path, maxPathLength); err != nil {
		return err
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

func getCurrentDatetime(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}
	return time.Now().Format(time.RFC3339), nil
}

func executeShellCommand(ctx context.Context, args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'command' parameter (use write_file for file writes)")
	}

	// Validate command for security
	if err := validateCommand(command); err != nil {
		return "", fmt.Errorf("command validation failed: %v", err)
	}

	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	return executeShellCommandHost(ctx, command)
}

func executeShellCommandHost(ctx context.Context, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out")
	}
	if ctx.Err() == context.Canceled {
		return string(output), fmt.Errorf("command canceled")
	}

	if err != nil {
		return string(output), fmt.Errorf("command failed: %v", err)
	}

	return string(output), nil
}

func readFile(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}

	workdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %v", err)
	}

	resolved, err := resolvePathWithinBase(path, workdir)
	if err != nil {
		return "", err
	}

	limits := getLimits()
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	if info.Size() > limits.MaxFileSizeBytes {
		return "", fmt.Errorf("file exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
	}

	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	// Use os.ReadFile instead of exec for better security
	content, err := os.ReadFile(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	if !isTextContent(content) {
		return "", fmt.Errorf("file appears to be binary; read_file supports text only")
	}

	return string(content), nil
}

func writeFile(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	path, err := extractPathArg(args)
	if err != nil {
		return "", err
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'content' parameter")
	}

	limits := getLimits()
	if int64(len(content)) > limits.MaxFileSizeBytes {
		return "", fmt.Errorf("content exceeds maximum size of %d bytes", limits.MaxFileSizeBytes)
	}

	if !isTextContent([]byte(content)) {
		return "", fmt.Errorf("content appears to be binary; write_file supports text only")
	}

	workdir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %v", err)
	}

	resolved, err := resolvePathWithinBase(path, workdir)
	if err != nil {
		return "", err
	}

	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return "", fmt.Errorf("path '%s' is a directory", resolved)
	}

	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	// Use os.WriteFile instead of exec for better security
	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), resolved), nil
}

func listDirectory(ctx context.Context, args map[string]interface{}) (string, error) {
	if err := ensureContext(ctx); err != nil {
		return "", err
	}

	path := getPathArg(args)
	if err := validateDirectoryPath(path); err != nil {
		return "", err
	}

	recursive := getBoolArg(args, "recursive")
	showHidden := getBoolArg(args, "show_hidden")
	limits := getLimits()

	var result strings.Builder
	var err error

	if recursive {
		err = walkDirectory(ctx, path, showHidden, limits, &result)
	} else {
		err = listDirectoryNonRecursive(ctx, path, showHidden, limits, &result)
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
	if err := paths.ValidatePathString(path, maxPathLength); err != nil {
		return "", err
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

	baseAbs, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %v", err)
	}
	baseResolved, err := filepath.EvalSymlinks(baseAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %v", err)
	}
	if err := validatePathWhitelist(absPath, baseResolved); err != nil {
		return "", err
	}

	return absPath, nil
}

func resolvePathWithinBase(path, baseDir string) (string, error) {
	if err := paths.ValidatePathString(path, maxPathLength); err != nil {
		return "", err
	}

	resolved, err := paths.ResolveWithinBase(path, baseDir)
	if err != nil {
		return "", err
	}

	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(resolved, dangerous) {
			return "", fmt.Errorf("access to %s is restricted for security", dangerous)
		}
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %v", err)
	}
	baseResolved, err := filepath.EvalSymlinks(baseAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %v", err)
	}
	if err := validatePathWhitelist(resolved, baseResolved); err != nil {
		return "", err
	}

	return resolved, nil
}

func validatePathWhitelist(absPath, baseResolved string) error {
	whitelist := getPathWhitelist()
	if len(whitelist) == 0 {
		return nil
	}

	for _, entry := range whitelist {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		allowed, err := paths.ResolveWhitelistEntry(entry, baseResolved)
		if err != nil {
			return err
		}
		if paths.HasPathPrefix(absPath, allowed) {
			return nil
		}
	}

	return fmt.Errorf("path is outside allowed tool base directories")
}

func isTextContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if !utf8.Valid(data) {
		return false
	}

	const sampleSize = 8192
	limit := len(data)
	if limit > sampleSize {
		limit = sampleSize
	}

	var nonPrintable int
	for _, b := range data[:limit] {
		switch b {
		case '\n', '\r', '\t':
			continue
		}
		if b == 0 {
			return false
		}
		if b < 0x20 || b == 0x7f {
			nonPrintable++
		}
	}

	return nonPrintable*20 < limit
}

func walkDirectory(ctx context.Context, path string, showHidden bool, limits Limits, result *strings.Builder) error {
	dirEntryCounts := make(map[string]int)
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if ctxErr := ensureContext(ctx); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return err
		}

		depth, err := depthFromBase(path, filePath)
		if err != nil {
			return err
		}
		if depth > limits.MaxDirectoryDepth {
			if info.IsDir() {
				return fmt.Errorf("directory depth exceeds maximum of %d", limits.MaxDirectoryDepth)
			}
			return nil
		}

		if filePath != path {
			parent := filepath.Dir(filePath)
			if err := bumpDirectoryCount(dirEntryCounts, parent, limits.MaxDirectoryEntries); err != nil {
				return err
			}
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

func listDirectoryNonRecursive(ctx context.Context, path string, showHidden bool, limits Limits, result *strings.Builder) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err)
	}
	if len(entries) > limits.MaxDirectoryEntries {
		return fmt.Errorf("directory contains more than %d entries", limits.MaxDirectoryEntries)
	}

	for _, entry := range entries {
		if err := ensureContext(ctx); err != nil {
			return err
		}
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

func depthFromBase(basePath, filePath string) (int, error) {
	rel, err := filepath.Rel(basePath, filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to compute relative path: %v", err)
	}
	if rel == "." {
		return 0, nil
	}
	return strings.Count(rel, string(os.PathSeparator)) + 1, nil
}

func bumpDirectoryCount(dirEntryCounts map[string]int, dir string, limit int) error {
	if limit <= 0 {
		return nil
	}
	dirEntryCounts[dir]++
	if dirEntryCounts[dir] > limit {
		return fmt.Errorf("directory contains more than %d entries", limit)
	}
	return nil
}

func shouldSkipHidden(filePath string, info os.FileInfo, basePath string, showHidden bool) bool {
	return !showHidden && isHidden(filepath.Base(filePath)) && filePath != basePath
}

func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

func ensureContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// extractPathArg accepts a variety of shapes for the path argument and normalizes to string.
func extractPathArg(args map[string]interface{}) (string, error) {
	if args == nil {
		return "", fmt.Errorf("missing or invalid 'path' parameter")
	}

	if path, ok := getStringLike(args["path"]); ok {
		return path, nil
	}

	// Common alternate keys the model sometimes emits.
	if path, ok := getStringLike(args["file"]); ok {
		return path, nil
	}
	if path, ok := getStringLike(args["filepath"]); ok {
		return path, nil
	}

	return "", fmt.Errorf("missing or invalid 'path' parameter")
}

func getStringLike(val interface{}) (string, bool) {
	switch v := val.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", false
		}
		return v, true
	case []byte:
		if len(v) == 0 {
			return "", false
		}
		return string(v), true
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				return s, true
			}
		}
	case map[string]interface{}:
		if nested, ok := getStringLike(v["path"]); ok {
			return nested, true
		}
	}
	return "", false
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
