package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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

// Tool implementations

func getCurrentDatetime(args map[string]interface{}) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}

func executeShellCommand(args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid 'command' parameter")
	}

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
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

	cmd := exec.Command("cat", path)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	return string(output), nil
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

	cmd := exec.Command("sh", "-c", fmt.Sprintf("cat > %s", path))
	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}

func listDirectory(args map[string]interface{}) (string, error) {
	// Get path parameter (default to current directory)
	path, ok := args["path"].(string)
	if !ok || path == "" {
		path = "."
	}

	// Get recursive flag (default to false)
	recursive := false
	if recursiveArg, ok := args["recursive"].(bool); ok {
		recursive = recursiveArg
	}

	// Get show_hidden flag (default to false)
	showHidden := false
	if showHiddenArg, ok := args["show_hidden"].(bool); ok {
		showHidden = showHiddenArg
	}

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("path not found: %v", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path '%s' is not a directory", path)
	}

	var result strings.Builder

	if recursive {
		// Use filepath.Walk for recursive listing
		err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip hidden files if not requested
			if !showHidden && strings.HasPrefix(filepath.Base(filePath), ".") && filePath != path {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Format entry
			formatEntry(&result, filePath, info, path)
			return nil
		})

		if err != nil {
			return "", fmt.Errorf("failed to walk directory: %v", err)
		}
	} else {
		// Use os.ReadDir for non-recursive listing
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %v", err)
		}

		for _, entry := range entries {
			// Skip hidden files if not requested
			if !showHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			// Get full file info
			entryPath := filepath.Join(path, entry.Name())
			info, err := entry.Info()
			if err != nil {
				result.WriteString(fmt.Sprintf("%-40s <error reading info>\n", entry.Name()))
				continue
			}

			formatEntry(&result, entryPath, info, path)
		}
	}

	output := result.String()
	if output == "" {
		return "Directory is empty", nil
	}

	return output, nil
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
