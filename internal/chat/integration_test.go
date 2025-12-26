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

package chat

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sashabaranov/go-openai"
	"promptline/internal/config"
	"promptline/internal/tools"
)

func TestToolApprovalWorkflowIntegration(t *testing.T) {
	dir, err := os.MkdirTemp(".", "chat-integration-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	relPath, err := filepath.Rel(".", path)
	if err != nil {
		t.Fatalf("failed to get relative path: %v", err)
	}

	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
		Tools: config.ToolSettings{
			Ask: []string{"read_file"},
		},
	}
	session := NewSessionWithClient(cfg, &MockChatClient{})
	session.ToolApprover = func(call openai.ToolCall) (bool, error) {
		return call.Function.Name == "read_file", nil
	}

	readArgs, err := json.Marshal(map[string]interface{}{
		"path": relPath,
	})
	if err != nil {
		t.Fatalf("failed to marshal read args: %v", err)
	}
	readCall := openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      "read_file",
			Arguments: string(readArgs),
		},
	}
	readResult := session.ExecuteToolCallWithApproval(readCall)
	if readResult.Error != nil {
		t.Fatalf("expected read_file to be approved, got %v", readResult.Error)
	}
	if readResult.Result != "hello" {
		t.Fatalf("expected read content, got %q", readResult.Result)
	}

	writeArgs, err := json.Marshal(map[string]interface{}{
		"path":    relPath,
		"content": "updated",
	})
	if err != nil {
		t.Fatalf("failed to marshal write args: %v", err)
	}
	writeCall := openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      "write_file",
			Arguments: string(writeArgs),
		},
	}
	writeResult := session.ExecuteToolCallWithApproval(writeCall)
	if writeResult.Error == nil {
		t.Fatal("expected write_file to be denied")
	}
	if !errors.Is(writeResult.Error, tools.ErrToolNotAllowed) && !errors.Is(writeResult.Error, tools.ErrToolDeniedByUser) {
		t.Fatalf("expected permission error, got %v", writeResult.Error)
	}
}
