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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/sashabaranov/go-openai"
	"promptline/internal/chat"
	"promptline/internal/config"
	"promptline/internal/tools"
)

func TestExecuteToolCallSuccess(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	colors := testColorScheme()

	toolCall := &openai.ToolCall{
		ID:   "call_123",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "get_current_datetime",
			Arguments: "{}",
		},
	}

	// Should not panic
	success := executeToolCall(session, toolCall, colors, logger)
	if !success {
		t.Fatalf("expected tool call success")
	}

	// Verify tool result was added to history
	history := session.GetHistory()
	if len(history) == 0 {
		t.Error("Expected tool result in history")
	}
}

func TestExecuteToolCallWithArgs(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	colors := testColorScheme()

	toolCall := &openai.ToolCall{
		ID:   "call_456",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "nonexistent_tool",
			Arguments: `{"path": "."}`,
		},
	}

	if executeToolCall(session, toolCall, colors, logger) {
		t.Fatalf("expected failure for nonexistent tool")
	}
}

func TestExecuteToolCallLsSuccess(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	colors := testColorScheme()

	// Call non-existent tool
	toolCall := &openai.ToolCall{
		ID:   "call_789",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: "{}",
		},
	}

	if !executeToolCall(session, toolCall, colors, logger) {
		t.Fatalf("expected success for allowed ls tool")
	}

	history := session.GetHistory()
	if len(history) == 0 {
		t.Error("Expected result in history")
	}
}

func TestExecuteToolCallLongResult(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := chat.NewSession(cfg)
	logger := zerolog.Nop()
	colors := testColorScheme()

	// Create a tool call that will generate a long result
	toolCall := &openai.ToolCall{
		ID:   "call_long",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: `{"path": ".", "recursive": false}`,
		},
	}

	// Should truncate long results in display
	executeToolCall(session, toolCall, colors, logger)
}

func TestToolsFormatToolResult(t *testing.T) {
	toolCall := openai.ToolCall{
		ID:   "call_test",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "test_tool",
			Arguments: `{"arg": "value"}`,
		},
	}

	result := &tools.ToolResult{
		Function: "test_tool",
		Result:   "Success",
		Error:    nil,
	}

	formatted := tools.FormatToolResult(toolCall, result, false)

	if formatted == "" {
		t.Error("Expected non-empty formatted result")
	}

	if !contains(formatted, "test_tool") {
		t.Error("Expected formatted result to contain tool name")
	}

	if !contains(formatted, "Success") {
		t.Error("Expected formatted result to contain result")
	}
}

func TestToolsFormatToolResultWithTruncation(t *testing.T) {
	toolCall := openai.ToolCall{
		ID:   "call_test",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "test_tool",
			Arguments: "{}",
		},
	}

	// Create a long result (>200 chars)
	longResult := make([]byte, 300)
	for i := range longResult {
		longResult[i] = 'a'
	}

	result := &tools.ToolResult{
		Function: "test_tool",
		Result:   string(longResult),
		Error:    nil,
	}

	formatted := tools.FormatToolResult(toolCall, result, true)

	// Should contain truncation indicator
	if !contains(formatted, "...") {
		t.Error("Expected truncation indicator in long result")
	}
}

func TestToolsFormatToolResultWithError(t *testing.T) {
	toolCall := openai.ToolCall{
		ID:   "call_test",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "test_tool",
			Arguments: "{}",
		},
	}

	result := &tools.ToolResult{
		Function: "test_tool",
		Result:   "",
		Error:    tools.ErrToolNotAllowed,
	}

	formatted := tools.FormatToolResult(toolCall, result, false)

	if !contains(formatted, "Error") && !contains(formatted, "error") {
		t.Error("Expected formatted result to indicate error")
	}
}

func TestExecuteToolCallFillsMissingPathFromHistory(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(prev)
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := chat.NewSession(cfg)
	session.AddMessage("user", "read config.json")

	logger := zerolog.Nop()
	colors := testColorScheme()

	toolCall := &openai.ToolCall{
		ID:   "call_missing_path",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "read_file",
			Arguments: "{}",
		},
	}

	executeToolCall(session, toolCall, colors, logger)

	history := session.GetHistory()
	if len(history) == 0 {
		t.Fatalf("expected tool result in history")
	}
	last := history[len(history)-1]
	if strings.TrimSpace(last.Content) != "hello" {
		t.Fatalf("expected file content, got: %s", last.Content)
	}
}

func TestExecuteToolCallFillsPathFromAssistantMention(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(filePath, []byte("assistant seen"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(prev)
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := chat.NewSession(cfg)
	session.AddMessage("user", "can you see the config")
	session.AddAssistantMessage("Yes, I can see `config.json`.", nil)

	logger := zerolog.Nop()
	colors := testColorScheme()

	toolCall := &openai.ToolCall{
		ID:   "call_assistant_path",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "read_file",
			Arguments: "{}",
		},
	}

	executeToolCall(session, toolCall, colors, logger)

	history := session.GetHistory()
	if len(history) == 0 {
		t.Fatalf("expected tool result in history")
	}
	last := history[len(history)-1]
	if strings.TrimSpace(last.Content) != "assistant seen" {
		t.Fatalf("expected file content, got: %s", last.Content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsRecursive(s, substr)
}

func containsRecursive(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s[:len(substr)] == substr {
		return true
	}
	return containsRecursive(s[1:], substr)
}
