package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func TestExecuteListDirectory(t *testing.T) {
	registry := NewRegistry()
	tempDir := t.TempDir()

	// create a file to ensure output is non-empty
	filePath := filepath.Join(tempDir, "example.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	result := registry.Execute("ls", map[string]interface{}{
		"path": tempDir,
	})

	if result.Error != nil {
		t.Fatalf("expected no error, got: %v", result.Error)
	}
	if result.Result == "" {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result.Result, "example.txt") {
		t.Fatalf("expected output to include created file, got: %s", result.Result)
	}
}

func TestExecuteUnknownTool(t *testing.T) {
	registry := NewRegistry()
	result := registry.Execute("does_not_exist", nil)
	if result.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestExecuteOpenAIToolCall(t *testing.T) {
	registry := NewRegistry()
	tempDir := t.TempDir()

	args := `{"path": "` + tempDir + `"}`
	call := openai.ToolCall{
		ID:   "call-1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: args,
		},
	}

	result := registry.ExecuteOpenAIToolCall(call)
	if result.Error != nil {
		t.Fatalf("expected no error, got: %v", result.Error)
	}
}

func TestExecuteOpenAIToolCallInvalidArgs(t *testing.T) {
	registry := NewRegistry()
	call := openai.ToolCall{
		ID:   "call-1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: `{"path": `, // invalid JSON
		},
	}
	result := registry.ExecuteOpenAIToolCall(call)
	if result.Error == nil {
		t.Fatal("expected error for invalid JSON arguments")
	}
}

func TestExecuteOpenAIToolCallMissingName(t *testing.T) {
	registry := NewRegistry()
	call := openai.ToolCall{
		ID:   "call-1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "",
			Arguments: `{"path": "."}`,
		},
	}
	result := registry.ExecuteOpenAIToolCall(call)
	if result.Error == nil {
		t.Fatal("expected error for missing function name")
	}
	if result.Function != "unknown_tool" {
		t.Fatalf("expected function to default to unknown_tool, got %s", result.Function)
	}
}

func TestExecuteGetCurrentDatetime(t *testing.T) {
	registry := NewRegistry()
	result := registry.Execute("get_current_datetime", map[string]interface{}{})
	if result.Error != nil {
		t.Fatalf("expected no error, got: %v", result.Error)
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(result.Result)); err != nil {
		t.Fatalf("expected RFC3339 time, got: %s (err: %v)", result.Result, err)
	}
}

func TestExecuteShellCommand(t *testing.T) {
	registry := NewRegistry()
	result := registry.Execute("execute_shell_command", map[string]interface{}{
		"command": "printf 'hello'",
	})
	if result.Error != nil {
		t.Fatalf("expected no error, got: %v", result.Error)
	}
	if strings.TrimSpace(result.Result) != "hello" {
		t.Fatalf("expected output 'hello', got %q", result.Result)
	}
}

func TestExecuteWriteAndReadFile(t *testing.T) {
	registry := NewRegistry()
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sample.txt")

	writeResult := registry.Execute("write_file", map[string]interface{}{
		"path":    filePath,
		"content": "sample content",
	})
	if writeResult.Error != nil {
		t.Fatalf("expected write_file success, got: %v", writeResult.Error)
	}

	readResult := registry.Execute("read_file", map[string]interface{}{
		"path": filePath,
	})
	if readResult.Error != nil {
		t.Fatalf("expected read_file success, got: %v", readResult.Error)
	}
	if strings.TrimSpace(readResult.Result) != "sample content" {
		t.Fatalf("expected content 'sample content', got %q", readResult.Result)
	}
}
