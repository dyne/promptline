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
	"errors"
	"io"

	"github.com/sashabaranov/go-openai"
)

// MockChatClient is a mock implementation of ChatClient for testing.
type MockChatClient struct {
	// Functions to override behavior
	CreateCompletionFunc       func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateCompletionStreamFunc func(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error)
	
	// Call tracking
	CompletionCalls      []openai.ChatCompletionRequest
	CompletionStreamCalls []openai.ChatCompletionRequest
}

// CreateChatCompletion implements ChatClient.
func (m *MockChatClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	m.CompletionCalls = append(m.CompletionCalls, req)
	if m.CreateCompletionFunc != nil {
		return m.CreateCompletionFunc(ctx, req)
	}
	// Default mock response
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
}

// CreateChatCompletionStream implements ChatClient.
func (m *MockChatClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	m.CompletionStreamCalls = append(m.CompletionStreamCalls, req)
	if m.CreateCompletionStreamFunc != nil {
		return m.CreateCompletionStreamFunc(ctx, req)
	}
	return nil, errors.New("mock stream not implemented")
}

// MockChatCompletionStream is a mock implementation for testing streaming.
type MockChatCompletionStream struct {
	Chunks []openai.ChatCompletionStreamResponse
	Index  int
	Closed bool
}

// Recv returns the next chunk or io.EOF when done.
func (m *MockChatCompletionStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	if m.Index >= len(m.Chunks) {
		return openai.ChatCompletionStreamResponse{}, io.EOF
	}
	chunk := m.Chunks[m.Index]
	m.Index++
	return chunk, nil
}

// Close marks the stream as closed.
func (m *MockChatCompletionStream) Close() error {
	m.Closed = true
	return nil
}
