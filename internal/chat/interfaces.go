package chat

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

// ChatClient interface abstracts the OpenAI client for testing.
// This enables dependency injection for unit tests without making real API calls.
//
// Usage:
//   - Production: use NewSession() which creates a real openai.Client
//   - Testing: use NewSessionWithClient() with a mock implementation
//
// Example:
//   mockClient := &MockChatClient{...}
//   session := NewSessionWithClient(cfg, mockClient)
type ChatClient interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error)
}

// HistoryStorage interface abstracts conversation history persistence.
// This can be used to implement alternative storage backends (database, cloud, etc.).
type HistoryStorage interface {
	Save(filepath string, messages []openai.ChatCompletionMessage) error
	Load(filepath string, maxLines int) ([]openai.ChatCompletionMessage, error)
}

// Verify that openai.Client implements ChatClient at compile time.
var _ ChatClient = (*openai.Client)(nil)
