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
