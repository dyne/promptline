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
	"context"
	"testing"

	"strings"
	"github.com/sashabaranov/go-openai"
	"promptline/internal/config"
)

// BenchmarkAddMessage measures message addition performance
func BenchmarkAddMessage(b *testing.B) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSession(cfg)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session.AddMessage(openai.ChatMessageRoleUser, "test message")
	}
}

// BenchmarkAddAssistantMessage measures assistant message addition
func BenchmarkAddAssistantMessage(b *testing.B) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSession(cfg)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session.AddAssistantMessage("response", nil)
	}
}

// BenchmarkMessagesSnapshot measures snapshot copy performance
func BenchmarkMessagesSnapshot(b *testing.B) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSession(cfg)
	
	// Add some messages
	for i := 0; i < 100; i++ {
		session.AddMessage(openai.ChatMessageRoleUser, "message")
		session.AddAssistantMessage("response", nil)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = session.MessagesSnapshot()
	}
}

// BenchmarkGetResponseWithMock measures response handling with mock client
func BenchmarkGetResponseWithMock(b *testing.B) {
	mockClient := &MockChatClient{
		CreateCompletionFunc: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			return openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Role:    openai.ChatMessageRoleAssistant,
							Content: "mock response",
						},
					},
				},
			}, nil
		},
	}
	
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSessionWithClient(cfg, mockClient)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = session.GetResponse("test input")
	}
}

// BenchmarkFinalizeToolCalls measures tool call finalization
func BenchmarkFinalizeToolCalls(b *testing.B) {
	toolCalls := map[string]*openai.ToolCall{
		"call1": {
			ID:   "call1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name: "test_tool",
			},
		},
		"call2": {
			ID:   "call2",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name: "another_tool",
			},
		},
	}
	
	argBuilders := make(map[string]*strings.Builder)
	for id := range toolCalls {
		builder := &strings.Builder{}
		builder.WriteString(`{"arg":"value"}`)
		argBuilders[id] = builder
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = finalizeToolCalls(toolCalls, argBuilders)
	}
}

// BenchmarkStreamEventCreation measures event creation overhead
func BenchmarkStreamEventCreation(b *testing.B) {
	b.Run("NewContentEvent", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewContentEvent("test content")
		}
	})
	
	b.Run("NewToolCallEvent", func(b *testing.B) {
		toolCall := &openai.ToolCall{
			ID:   "test",
			Type: openai.ToolTypeFunction,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = NewToolCallEvent(toolCall)
		}
	})
	
	b.Run("NewErrorEvent", func(b *testing.B) {
		err := &StreamError{Operation: "test", Err: context.Canceled}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = NewErrorEvent(err)
		}
	})
}

// BenchmarkConcurrentMessageAddition measures concurrent message additions
func BenchmarkConcurrentMessageAddition(b *testing.B) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSession(cfg)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			session.AddMessage(openai.ChatMessageRoleUser, "test")
		}
	})
}
