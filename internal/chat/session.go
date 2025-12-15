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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
	"promptline/internal/config"
	"promptline/internal/tools"
	systemprompt "promptline/system_prompt"
)

// Session represents a chat session with context.
//
// Thread-safety: Session is safe for concurrent use. All message operations
// (AddMessage, AddAssistantMessage, AddToolResultMessage, MessagesSnapshot,
// SaveConversationHistory, LoadConversationHistory) are protected by an internal
// mutex. The streaming methods (StreamResponseWithContext, processStream) create
// their own local state (toolCalls map, contentBuilder) and do not share mutable
// state between goroutines. ToolRegistry has its own thread-safety guarantees.
type Session struct {
	Client            ChatClient
	Config            *config.Config
	Messages          []openai.ChatCompletionMessage
	ToolRegistry      *tools.Registry
	mu                sync.Mutex
	lastSavedMsgCount int // Track how many messages were last saved (protected by mu)
}

var defaultSystemPrompt = mustLoadSystemPrompt()

func mustLoadSystemPrompt() string {
	prompt, err := loadSystemPrompt()
	if err != nil {
		panic(fmt.Sprintf("failed to load system prompt: %v", err))
	}
	return prompt
}

func loadSystemPrompt() (string, error) {
	return systemprompt.Load()
}

// NewSession creates a new chat session with a default OpenAI client.
func NewSession(cfg *config.Config) *Session {
	// Create client with custom base URL if provided
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.APIURL != "" {
		clientConfig.BaseURL = cfg.APIURL
		// For DashScope, we might need to set a custom HTTP client
		clientConfig.HTTPClient = &http.Client{}
	}

	client := openai.NewClientWithConfig(clientConfig)
	return NewSessionWithClient(cfg, client)
}

// NewSessionWithClient creates a new chat session with a provided client (for testing).
func NewSessionWithClient(cfg *config.Config, client ChatClient) *Session {
	// Initialize tool registry
	toolRegistry := tools.NewRegistryWithPolicy(cfg.ToolPolicy())

	systemPrompt := defaultSystemPrompt

	// Initialize with system message
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	sess := &Session{
		Client:       client,
		Config:       cfg,
		Messages:     messages,
		ToolRegistry: toolRegistry,
	}

	return sess
}

// AddMessage adds a message to the conversation history
func (s *Session) AddMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, openai.ChatCompletionMessage{
		Role:    role,
		Content: content,
	})
}

// AddAssistantMessage adds an assistant message with optional tool calls.
func (s *Session) AddAssistantMessage(content string, toolCalls []openai.ToolCall) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, openai.ChatCompletionMessage{
		Role:      openai.ChatMessageRoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
	})
}

// AddToolResultMessage appends a tool result message.
func (s *Session) AddToolResultMessage(call openai.ToolCall, result *tools.ToolResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use plain result as content. TOON encoding was causing issues with history serialization.
	// The API accepts plain text for tool results.
	content := result.Result
	if result.Error != nil {
		content = fmt.Sprintf("Error: %v", result.Error)
	}

	name := call.Function.Name
	if name == "" {
		name = "unknown_tool"
	}
	s.Messages = append(s.Messages, openai.ChatCompletionMessage{
		Role:       openai.ChatMessageRoleTool,
		Content:    content,
		Name:       name,
		ToolCallID: call.ID,
	})
}

// MessagesSnapshot returns a copy of the current messages.
func (s *Session) MessagesSnapshot() []openai.ChatCompletionMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := make([]openai.ChatCompletionMessage, len(s.Messages))
	copy(msgs, s.Messages)
	return msgs
}

// GetResponseWithContext gets a response from the OpenAI API
// Handles tool calls recursively until a final text response is received
func (s *Session) GetResponseWithContext(ctx context.Context, prompt string) (string, error) {
	s.AddMessage(openai.ChatMessageRoleUser, prompt)

	// Loop to handle tool calls
	for {
		req := openai.ChatCompletionRequest{
			Model:    s.Config.Model,
			Messages: s.MessagesSnapshot(),
			Tools:    s.ToolRegistry.OpenAITools(),
		}

		if s.Config.Temperature != nil {
			req.Temperature = *s.Config.Temperature
		}

		if s.Config.MaxTokens != nil {
			req.MaxTokens = *s.Config.MaxTokens
		}

		resp, err := s.Client.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", &APIError{Operation: "create_completion", Err: err}
		}

		response := resp.Choices[0].Message
		s.AddAssistantMessage(response.Content, response.ToolCalls)

		// If no tool calls, return the response
		if len(response.ToolCalls) == 0 {
			return response.Content, nil
		}

		// Execute all tool calls
		for _, toolCall := range response.ToolCalls {
			result := s.ToolRegistry.ExecuteOpenAIToolCall(toolCall)
			s.AddToolResultMessage(toolCall, result)
		}

		// Loop continues to get next response with tool results
	}
}

// GetResponse gets a response from the OpenAI API
func (s *Session) GetResponse(prompt string) (string, error) {
	return s.GetResponseWithContext(context.Background(), prompt)
}

// StreamEventType identifies the type of streaming event.
type StreamEventType int

const (
	StreamEventContent StreamEventType = iota
	StreamEventToolCall
	StreamEventError
)

// StreamEvent represents a chunk of streamed data from the model.
type StreamEvent struct {
	Type     StreamEventType
	Content  string
	ToolCall *openai.ToolCall
	Err      error
}

// NewContentEvent creates a content streaming event.
func NewContentEvent(content string) StreamEvent {
	return StreamEvent{Type: StreamEventContent, Content: content}
}

// NewToolCallEvent creates a tool call streaming event.
func NewToolCallEvent(toolCall *openai.ToolCall) StreamEvent {
	return StreamEvent{Type: StreamEventToolCall, ToolCall: toolCall}
}

// NewErrorEvent creates an error streaming event.
func NewErrorEvent(err error) StreamEvent {
	return StreamEvent{Type: StreamEventError, Err: err}
}

// StreamResponseWithContext gets a streaming response from the OpenAI API and sends it through a channel of events.
// If includeUserMessage is true, the prompt is added as a user message before sending the request.
func (s *Session) StreamResponseWithContext(ctx context.Context, prompt string, includeUserMessage bool, events chan<- StreamEvent) {
	defer close(events)

	if includeUserMessage && prompt != "" {
		s.AddMessage(openai.ChatMessageRoleUser, prompt)
	}

	stream, err := s.createStream(ctx)
	if err != nil {
		events <- NewErrorEvent(&StreamError{Operation: "create_stream", Err: err})
		return
	}
	defer stream.Close()

	s.processStream(ctx, stream, events)
}

func (s *Session) createStream(ctx context.Context) (*openai.ChatCompletionStream, error) {
	req := openai.ChatCompletionRequest{
		Model:    s.Config.Model,
		Messages: s.MessagesSnapshot(),
		Stream:   true,
		Tools:    s.ToolRegistry.OpenAITools(),
	}

	if s.Config.Temperature != nil {
		req.Temperature = *s.Config.Temperature
	}

	if s.Config.MaxTokens != nil {
		req.MaxTokens = *s.Config.MaxTokens
	}

	return s.Client.CreateChatCompletionStream(ctx, req)
}

// processStream handles the streaming loop and local state accumulation.
// Thread-safety: The contentBuilder, toolCalls, and argBuilders are local to
// this function call and not shared with other goroutines, so no locking needed.
func (s *Session) processStream(ctx context.Context, stream *openai.ChatCompletionStream, events chan<- StreamEvent) {
	var contentBuilder strings.Builder
	toolCalls := make(map[string]*openai.ToolCall)
	argBuilders := make(map[string]*strings.Builder)

	for {
		select {
		case <-ctx.Done():
			events <- NewErrorEvent(ctx.Err())
			return
		default:
			response, err := stream.Recv()
			if err != nil {
				s.handleStreamEnd(err, &contentBuilder, toolCalls, argBuilders, events)
				return
			}

			if len(response.Choices) == 0 {
				continue
			}

			s.handleStreamChunk(response.Choices[0].Delta, &contentBuilder, toolCalls, argBuilders, events)
		}
	}
}

func (s *Session) handleStreamEnd(err error, contentBuilder *strings.Builder, toolCalls map[string]*openai.ToolCall, argBuilders map[string]*strings.Builder, events chan<- StreamEvent) {
	if err == io.EOF {
		finalCalls := finalizeToolCalls(toolCalls, argBuilders)
		s.AddAssistantMessage(contentBuilder.String(), finalCalls)
		s.emitToolCalls(finalCalls, events)
		return
	}
	events <- NewErrorEvent(&StreamError{Operation: "receive_chunk", Err: err})
}

func (s *Session) handleStreamChunk(delta openai.ChatCompletionStreamChoiceDelta, contentBuilder *strings.Builder, toolCalls map[string]*openai.ToolCall, argBuilders map[string]*strings.Builder, events chan<- StreamEvent) {
	if delta.Content != "" {
		contentBuilder.WriteString(delta.Content)
		events <- NewContentEvent(delta.Content)
	}

	for _, tc := range delta.ToolCalls {
		entry := accumulateToolCall(toolCalls, argBuilders, tc)
		if entry != nil {
			toolCalls[tc.ID] = entry
		}
	}
}

func (s *Session) emitToolCalls(finalCalls []openai.ToolCall, events chan<- StreamEvent) {
	for _, call := range finalCalls {
		callCopy := call
		events <- NewToolCallEvent(&callCopy)
	}
}

// accumulateToolCall merges incremental tool call deltas into a stored call.
func accumulateToolCall(toolCalls map[string]*openai.ToolCall, argBuilders map[string]*strings.Builder, tc openai.ToolCall) *openai.ToolCall {
	entry, ok := toolCalls[tc.ID]
	if !ok {
		entry = &openai.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: openai.FunctionCall{
				Name: tc.Function.Name,
			},
		}
	}
	if entry.Function.Name == "" && tc.Function.Name != "" {
		entry.Function.Name = tc.Function.Name
	}

	builder, ok := argBuilders[tc.ID]
	if !ok {
		builder = &strings.Builder{}
		argBuilders[tc.ID] = builder
	}
	builder.WriteString(tc.Function.Arguments)
	// Don't update Arguments here - wait for finalization to avoid repeated string allocations

	return entry
}

// finalizeToolCalls ensures tool calls have names and JSON arguments.
func finalizeToolCalls(toolCalls map[string]*openai.ToolCall, argBuilders map[string]*strings.Builder) []openai.ToolCall {
	finalCalls := make([]openai.ToolCall, 0, len(toolCalls))
	for _, call := range toolCalls {
		rawArgs := ""
		if builder, ok := argBuilders[call.ID]; ok {
			rawArgs = builder.String()
		}
		trimmed := strings.TrimSpace(rawArgs)

		// Drop tool calls without IDs (malformed/incomplete)
		if call.ID == "" {
			continue
		}

		// Drop nameless + empty-arg tool calls (often stray/unsolicited).
		if call.Function.Name == "" && trimmed == "" {
			continue
		}

		args := rawArgs
		if trimmed == "" {
			args = "{}"
		} else if !json.Valid([]byte(args)) {
			args = "{}"
		}
		call.Function.Arguments = args
		if call.Function.Name == "" {
			call.Function.Name = "unknown_tool"
		}
		// Ensure type is set to function if empty
		if call.Type == "" {
			call.Type = openai.ToolTypeFunction
		}
		finalCalls = append(finalCalls, *call)
	}
	return finalCalls
}

// GetStreamingResponseWithContext gets a streaming response from the OpenAI API and prints it.
func (s *Session) GetStreamingResponseWithContext(ctx context.Context, prompt string) error {
	return s.streamAndPrint(ctx, prompt, true)
}

func (s *Session) streamAndPrint(ctx context.Context, prompt string, includeUserMessage bool) error {
	events := make(chan StreamEvent)
	go s.StreamResponseWithContext(ctx, prompt, includeUserMessage, events)

	fmt.Print("Assistant: ")
	for event := range events {
		switch event.Type {
		case StreamEventContent:
			fmt.Print(event.Content)
		case StreamEventToolCall:
			if event.ToolCall == nil {
				continue
			}
			result := s.ToolRegistry.ExecuteOpenAIToolCall(*event.ToolCall)
			s.AddToolResultMessage(*event.ToolCall, result)
			fmt.Printf("\n%s\n", s.FormatToolCallDisplay(*event.ToolCall, result))
			// Request a follow-up response without adding another user message
			return s.streamAndPrint(ctx, "", false)
		case StreamEventError:
			return event.Err
		}
	}

	fmt.Println()
	return nil
}

// ClearHistory clears the conversation history
func (s *Session) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	systemMsg := s.Messages[0]
	s.Messages = []openai.ChatCompletionMessage{systemMsg}
}

// GetHistory returns the conversation history excluding system message
func (s *Session) GetHistory() []openai.ChatCompletionMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Messages) <= 1 {
		return []openai.ChatCompletionMessage{}
	}
	return s.Messages[1:]
}

// SaveConversationHistory appends new messages to the history file
func (s *Session) SaveConversationHistory(filepath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Only save non-system messages
	history := s.Messages[1:]

	// Check if there are new messages to save
	if len(history) <= s.lastSavedMsgCount {
		return nil // Nothing new to save
	}

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return &HistoryError{Operation: "open", Filepath: filepath, Err: err}
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	// Only save messages we haven't saved yet
	for i := s.lastSavedMsgCount; i < len(history); i++ {
		if err := encoder.Encode(history[i]); err != nil {
			return &HistoryError{Operation: "encode", Filepath: filepath, Err: err}
		}
	}

	s.lastSavedMsgCount = len(history)
	return nil
}

// LoadConversationHistory loads conversation history from a file with a line limit
func (s *Session) LoadConversationHistory(filepath string, maxLines int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No history file is okay
		}
		return &HistoryError{Operation: "open", Filepath: filepath, Err: err}
	}
	defer file.Close()

	// Read all lines
	var messages []openai.ChatCompletionMessage
	decoder := json.NewDecoder(file)
	for {
		var msg openai.ChatCompletionMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return &HistoryError{Operation: "decode", Filepath: filepath, Err: err}
		}
		messages = append(messages, msg)
	}

	// Apply limit - keep only the last N messages
	if maxLines > 0 && len(messages) > maxLines {
		messages = messages[len(messages)-maxLines:]
	}

	// Append to session (after system message)
	s.Messages = append(s.Messages, messages...)

	// Update saved message count since we loaded them
	s.lastSavedMsgCount = len(messages)

	return nil
}

// PrintHistory prints the conversation history
func (s *Session) PrintHistory() {
	fmt.Println("--- Conversation History ---")
	for _, msg := range s.MessagesSnapshot() {
		role := "Unknown"
		switch msg.Role {
		case openai.ChatMessageRoleSystem:
			role = "System"
		case openai.ChatMessageRoleUser:
			role = "User"
		case openai.ChatMessageRoleAssistant:
			role = "Assistant"
		case openai.ChatMessageRoleTool:
			role = "Tool"
		}
		fmt.Printf("%s: %s\n", role, msg.Content)
	}
	fmt.Println("--- End History ---")
}

// FormatToolCallDisplay creates a user-friendly display of tool execution
// Deprecated: Use tools.FormatToolResult instead
func (s *Session) FormatToolCallDisplay(toolCall openai.ToolCall, result *tools.ToolResult) string {
	return tools.FormatToolResult(toolCall, result, false)
}

// Close is a no-op for compatibility but may be used for cleanup in the future
func (s *Session) Close() error {
	return nil
}
