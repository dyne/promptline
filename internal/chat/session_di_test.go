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

	"github.com/sashabaranov/go-openai"
	"promptline/internal/config"
)

func TestNewSessionWithClient(t *testing.T) {
	mockClient := &MockChatClient{}
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	
	session := NewSessionWithClient(cfg, mockClient)
	
	if session == nil {
		t.Fatal("expected session to be created")
	}
	if session.Client != mockClient {
		t.Error("expected session to use mock client")
	}
	if session.Config != cfg {
		t.Error("expected session to use provided config")
	}
	if len(session.Messages) != 1 {
		t.Errorf("expected 1 system message, got %d", len(session.Messages))
	}
}

func TestGetResponseWithMockClient(t *testing.T) {
	mockClient := &MockChatClient{
		CreateCompletionFunc: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
			return openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Role:    openai.ChatMessageRoleAssistant,
							Content: "Hello from mock!",
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
	
	response, err := session.GetResponse("Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if response != "Hello from mock!" {
		t.Errorf("expected 'Hello from mock!', got %q", response)
	}
	
	// Verify the mock was called
	if len(mockClient.CompletionCalls) != 1 {
		t.Errorf("expected 1 completion call, got %d", len(mockClient.CompletionCalls))
	}
	
	// Verify user message was added
	if len(session.Messages) != 3 { // system + user + assistant
		t.Errorf("expected 3 messages, got %d", len(session.Messages))
	}
}

func TestStreamResponseWithMockClient(t *testing.T) {
	mockClient := &MockChatClient{
		CreateCompletionStreamFunc: func(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
			// Return a mock stream - note: this won't work directly because ChatCompletionStream is a concrete type
			// This test demonstrates the interface pattern even though streaming needs more work
			return nil, nil
		},
	}
	
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSessionWithClient(cfg, mockClient)
	
	// Just verify the mock client is being used
	if session.Client != mockClient {
		t.Error("expected session to use mock client")
	}
	
	// Note: Full streaming test would need the ChatCompletionStream to be an interface
	// For now, we've demonstrated the dependency injection pattern for the main client
}

func TestMockClientCallTracking(t *testing.T) {
	mockClient := &MockChatClient{}
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}
	session := NewSessionWithClient(cfg, mockClient)
	
	// Make multiple calls
	_, _ = session.GetResponse("First message")
	_, _ = session.GetResponse("Second message")
	
	if len(mockClient.CompletionCalls) != 2 {
		t.Errorf("expected 2 completion calls, got %d", len(mockClient.CompletionCalls))
	}
	
	// Verify the requests had the right messages
	if len(mockClient.CompletionCalls[0].Messages) != 2 { // system + user
		t.Errorf("expected first call to have 2 messages, got %d", len(mockClient.CompletionCalls[0].Messages))
	}
	
	if len(mockClient.CompletionCalls[1].Messages) != 4 { // system + user + assistant + user
		t.Errorf("expected second call to have 4 messages, got %d", len(mockClient.CompletionCalls[1].Messages))
	}
}
