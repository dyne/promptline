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
