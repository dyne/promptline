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
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestExecuteOpenAIToolCallCreateReadIntegration(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Allow: map[string]bool{
			"create_file": true,
			"read_file":   true,
		},
	})

	dir, err := os.MkdirTemp(".", "tools-integration-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	relPath, err := filepath.Rel(".", filepath.Join(dir, "sample.txt"))
	if err != nil {
		t.Fatalf("failed to compute relative path: %v", err)
	}

	writeArgs, _ := json.Marshal(map[string]interface{}{
		"path":    relPath,
		"content": "integration content",
	})
	writeCall := openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      "create_file",
			Arguments: string(writeArgs),
		},
	}
	writeResult := registry.ExecuteOpenAIToolCall(writeCall)
	if writeResult.Error != nil {
		t.Fatalf("expected create_file success, got %v", writeResult.Error)
	}

	readArgs, _ := json.Marshal(map[string]interface{}{
		"path": relPath,
	})
	readCall := openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      "read_file",
			Arguments: string(readArgs),
		},
	}
	readResult := registry.ExecuteOpenAIToolCall(readCall)
	if readResult.Error != nil {
		t.Fatalf("expected read_file success, got %v", readResult.Error)
	}
	if readResult.Result != "integration content" {
		t.Fatalf("expected content to match, got %q", readResult.Result)
	}
}

func TestExecuteOpenAIToolCallRequiresConfirmationIntegration(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Ask: map[string]bool{
			"get_current_datetime": true,
		},
	})

	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      "get_current_datetime",
			Arguments: `{}`,
		},
	}
	result := registry.ExecuteOpenAIToolCall(call)
	if result.Error == nil {
		t.Fatal("expected confirmation error")
	}
	if !errors.Is(result.Error, ErrToolRequiresConfirmation) {
		t.Fatalf("expected ErrToolRequiresConfirmation, got %v", result.Error)
	}
}

func TestCreateFileValidationIntegration(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Allow: map[string]bool{
			"create_file": true,
		},
	})

	result := registry.Execute("create_file", map[string]interface{}{
		"content": "missing path",
	})
	if result.Error == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(result.Error.Error(), "missing or invalid 'path' parameter") {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}

func TestCreateFileCustomValidationIntegration(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Allow: map[string]bool{
			"create_file": true,
		},
	})

	result := registry.Execute("create_file", map[string]interface{}{
		"path":    "../escape.txt",
		"content": "content",
	})
	if result.Error == nil {
		t.Fatal("expected path validation error")
	}
	if !strings.Contains(result.Error.Error(), "path escapes working directory") {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}
