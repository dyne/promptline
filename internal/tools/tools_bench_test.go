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
	"testing"

	"github.com/sashabaranov/go-openai"
)

// BenchmarkToolRegistration measures tool registration performance
func BenchmarkToolRegistration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry := NewRegistry()
		_ = registry
	}
}

// BenchmarkGetPermission measures permission check performance
func BenchmarkGetPermission(b *testing.B) {
	registry := NewRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.getPermission("ls")
	}
}

// BenchmarkExecuteOpenAIToolCall measures tool execution overhead
func BenchmarkExecuteOpenAIToolCall(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("ls", true)

	toolCall := openai.ToolCall{
		ID:   "test-call",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: `{"path": "."}`,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.ExecuteOpenAIToolCall(toolCall)
	}
}

// BenchmarkExecuteMultipleTools measures batch tool execution
func BenchmarkExecuteMultipleTools(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("ls", true)
	registry.SetAllowed("get_current_datetime", true)

	toolCalls := []openai.ToolCall{
		{
			ID:   "call1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "ls",
				Arguments: `{"path": "."}`,
			},
		},
		{
			ID:   "call2",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "get_current_datetime",
				Arguments: `{}`,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range toolCalls {
			_ = registry.ExecuteOpenAIToolCall(tc)
		}
	}
}

// BenchmarkFormatToolResult measures result formatting performance
func BenchmarkFormatToolResult(b *testing.B) {
	toolCall := openai.ToolCall{
		ID:   "test-call",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: `{"path": "."}`,
		},
	}

	result := &ToolResult{
		Function: "ls",
		Result:   "file1.txt\nfile2.txt\nfile3.txt",
		Error:    nil,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatToolResult(toolCall, result, false)
	}
}

// BenchmarkConcurrentToolExecution measures concurrent tool execution
func BenchmarkConcurrentToolExecution(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("get_current_datetime", true)

	toolCall := openai.ToolCall{
		ID:   "test-call",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "get_current_datetime",
			Arguments: `{}`,
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = registry.ExecuteOpenAIToolCall(toolCall)
		}
	})
}

// BenchmarkPolicyApplicationBench measures policy application overhead
func BenchmarkPolicyApplicationBench(b *testing.B) {
	policy := Policy{
		Allow: map[string]bool{
			"tool1": true,
			"tool2": true,
		},
		Ask: map[string]bool{
			"tool2": true,
		},
		Deny: map[string]bool{
			"tool3": true,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry := NewRegistry()
		registry.applyPolicy(policy)
	}
}

// BenchmarkOpenAIToolsConversion measures OpenAI tools conversion
func BenchmarkOpenAIToolsConversion(b *testing.B) {
	registry := NewRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.OpenAITools()
	}
}
