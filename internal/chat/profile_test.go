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
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
	"promptline/internal/tools"
	"promptline/internal/config"
)

// BenchmarkStreamProcessing measures streaming with tool calls
func BenchmarkStreamProcessing(b *testing.B) {
	for i := 0; i < b.N; i++ {
		toolCalls := make(map[string]*openai.ToolCall)
		argBuilders := make(map[string]*strings.Builder)
		
		// Simulate streaming chunks
		for j := 0; j < 10; j++ {
			tc := openai.ToolCall{
				ID:   "call1",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      "test_tool",
					Arguments: `{"arg":`,
				},
			}
			accumulateToolCall(toolCalls, argBuilders, tc)
		}
		
		_ = finalizeToolCalls(toolCalls, argBuilders)
	}
}

// BenchmarkHistorySave measures history save performance
func BenchmarkHistorySave(b *testing.B) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSession(cfg)
	
	// Add 50 messages
	for i := 0; i < 50; i++ {
		session.AddMessage(openai.ChatMessageRoleUser, "test message")
		session.AddAssistantMessage("response", nil)
	}
	
	tmpFile := b.TempDir() + "/history.jsonl"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = session.SaveConversationHistory(tmpFile)
	}
}

// BenchmarkHistoryLoad measures history load performance
func BenchmarkHistoryLoad(b *testing.B) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSession(cfg)
	
	// Create history file
	for i := 0; i < 50; i++ {
		session.AddMessage(openai.ChatMessageRoleUser, "test message")
		session.AddAssistantMessage("response", nil)
	}
	
	tmpFile := b.TempDir() + "/history.jsonl"
	_ = session.SaveConversationHistory(tmpFile)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		newSession := NewSession(cfg)
		_ = newSession.LoadConversationHistory(tmpFile, 100)
	}
}

// BenchmarkToolResultProcessing measures tool result addition performance
func BenchmarkToolResultProcessing(b *testing.B) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSession(cfg)
	
	toolCall := openai.ToolCall{
		ID:   "test-call",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: `{"path": "."}`,
		},
	}
	
	result := &tools.ToolResult{
		Function: "ls",
		Result:   strings.Repeat("file.txt\n", 100), // Simulate large output
		Error:    nil,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session.AddToolResultMessage(toolCall, result)
	}
}
