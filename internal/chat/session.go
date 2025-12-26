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
	"time"

	"github.com/rs/zerolog"
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
	BaseURL           string
	ToolApprover      ToolApprovalFunc
	Logger            *zerolog.Logger
	mu                sync.Mutex
	lastSavedMsgCount int // Track how many messages were last saved (protected by mu)
}

// ToolApprovalFunc determines whether a tool call is approved for execution.
type ToolApprovalFunc func(call openai.ToolCall) (bool, error)

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
	sess := NewSessionWithClient(cfg, client)
	sess.BaseURL = clientConfig.BaseURL
	return sess
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
	if cfg.APIURL != "" {
		sess.BaseURL = cfg.APIURL
	} else {
		sess.BaseURL = openai.DefaultConfig("").BaseURL
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
	for i, msg := range s.Messages {
		msgs[i] = msg
		if len(msg.ToolCalls) == 0 {
			continue
		}
		toolCalls := make([]openai.ToolCall, len(msg.ToolCalls))
		for j, call := range msg.ToolCalls {
			if strings.TrimSpace(call.Function.Arguments) == "" {
				call.Function.Arguments = "{}"
			}
			toolCalls[j] = call
		}
		msgs[i].ToolCalls = toolCalls
	}
	return msgs
}

// GetResponseWithContext gets a response from the OpenAI API
// Handles tool calls recursively until a final text response is received
func (s *Session) GetResponseWithContext(ctx context.Context, prompt string) (string, error) {
	s.AddMessage(openai.ChatMessageRoleUser, prompt)

	// Loop to handle tool calls
	for {
		start := time.Now()
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

		s.debugLogRequest("create_completion", req)
		resp, err := s.Client.CreateChatCompletion(ctx, req)
		if err != nil {
			s.debugLogError("create_completion", err)
			return "", &APIError{Operation: "create_completion", Err: err}
		}
		s.debugLogCompletion("create_completion", time.Since(start), resp)

		response := resp.Choices[0].Message
		s.AddAssistantMessage(response.Content, response.ToolCalls)

		// If no tool calls, return the response
		if len(response.ToolCalls) == 0 {
			return response.Content, nil
		}

		// Execute all tool calls
		for _, toolCall := range response.ToolCalls {
			result := s.ExecuteToolCallWithApproval(toolCall)
			s.AddToolResultMessage(toolCall, result)
		}

		// Loop continues to get next response with tool results
	}
}

// ExecuteToolCallWithApproval evaluates tool permission and optionally asks for approval.
func (s *Session) ExecuteToolCallWithApproval(call openai.ToolCall) *tools.ToolResult {
	name := call.Function.Name
	if name == "" {
		return invalidToolResult("unknown_tool", fmt.Errorf("%w: tool call missing function name", tools.ErrInvalidArguments))
	}
	if err := s.preflightValidateToolCall(name, call.Function.Arguments); err != nil {
		return err
	}
	perm := s.ToolRegistry.GetPermission(name)
	if s.Logger != nil {
		s.Logger.Debug().
			Str("tool_name", name).
			Str("permission", string(perm.Level)).
			Msg("Tool permission evaluated")
	}
	switch perm.Level {
	case tools.PermissionAllow:
		return s.ToolRegistry.ExecuteOpenAIToolCall(call)
	case tools.PermissionDeny:
		return deniedToolResult(name, fmt.Sprintf("Tool %q is denied by policy.", name), tools.ErrToolNotAllowed)
	case tools.PermissionAsk:
		if s.ToolApprover == nil {
			return deniedToolResult(name, fmt.Sprintf("Tool %q requires user approval, but no approver is configured.", name), tools.ErrToolDeniedByUser)
		}
		if s.Logger != nil {
			s.Logger.Debug().
				Str("tool_name", name).
				Msg("Awaiting tool approval")
		}
		approved, err := s.ToolApprover(call)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Debug().
					Str("tool_name", name).
					Err(err).
					Msg("Tool approval failed")
			}
			return deniedToolResult(name, fmt.Sprintf("Tool %q approval failed: %v", name, err), tools.ErrToolDeniedByUser)
		}
		if !approved {
			if s.Logger != nil {
				s.Logger.Debug().
					Str("tool_name", name).
					Msg("Tool approval denied by user")
			}
			return deniedToolResult(name, fmt.Sprintf("User denied execution of tool %q.", name), tools.ErrToolDeniedByUser)
		}
		if s.Logger != nil {
			s.Logger.Debug().
				Str("tool_name", name).
				Msg("Tool approval granted")
		}
		return s.ToolRegistry.ExecuteOpenAIToolCallWithOptions(call, tools.ExecuteOptions{Force: true})
	default:
		return deniedToolResult(name, fmt.Sprintf("Tool %q requires user approval.", name), tools.ErrToolDeniedByUser)
	}
}

func deniedToolResult(name, message string, err error) *tools.ToolResult {
	return &tools.ToolResult{
		Function: name,
		Result:   message,
		Error:    err,
	}
}

func invalidToolResult(name string, err error) *tools.ToolResult {
	return &tools.ToolResult{
		Function: name,
		Result:   fmt.Sprintf("Error: %v", err),
		Error:    err,
	}
}

func (s *Session) preflightValidateToolCall(name, argsJSON string) *tools.ToolResult {
	if !s.toolExists(name) {
		return invalidToolResult(name, fmt.Errorf("%w: tool %q not found", tools.ErrToolNotFound, name))
	}

	args := map[string]interface{}{}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return invalidToolResult(name, fmt.Errorf("%w: %v", tools.ErrInvalidArguments, err))
		}
	}

	switch name {
	case "execute_shell_command":
		if !hasRequiredStringArg(args, "command") {
			return invalidToolResult(name, fmt.Errorf("%w: missing or invalid 'command' parameter (use write_file for file writes)", tools.ErrInvalidArguments))
		}
	case "write_file":
		if !hasRequiredStringArg(args, "path") || !hasRequiredStringArg(args, "content") {
			return invalidToolResult(name, fmt.Errorf("%w: missing or invalid 'path' or 'content' parameter", tools.ErrInvalidArguments))
		}
	case "read_file":
		if !hasNonEmptyArg(args, "path") {
			return invalidToolResult(name, fmt.Errorf("%w: missing or invalid 'path' parameter", tools.ErrInvalidArguments))
		}
	}

	return nil
}

func (s *Session) toolExists(name string) bool {
	for _, toolName := range s.ToolRegistry.GetToolNames() {
		if toolName == name {
			return true
		}
	}
	return false
}

func hasRequiredStringArg(args map[string]interface{}, key string) bool {
	value, ok := args[key]
	if !ok || value == nil {
		return false
	}
	str, ok := value.(string)
	return ok && strings.TrimSpace(str) != ""
}

func hasNonEmptyArg(args map[string]interface{}, key string) bool {
	value, ok := args[key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	default:
		return true
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

	start := time.Now()
	stream, err := s.createStream(ctx)
	if err != nil {
		s.debugLogError("create_stream", err)
		events <- NewErrorEvent(&StreamError{Operation: "create_stream", Err: err})
		return
	}
	defer stream.Close()

	s.processStream(ctx, stream, events, start)
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

	s.debugLogRequest("create_stream", req)
	return s.Client.CreateChatCompletionStream(ctx, req)
}

// processStream handles the streaming loop and local state accumulation.
// Thread-safety: The contentBuilder, toolCalls, and argBuilders are local to
// this function call and not shared with other goroutines, so no locking needed.
func (s *Session) processStream(ctx context.Context, stream *openai.ChatCompletionStream, events chan<- StreamEvent, start time.Time) {
	var contentBuilder strings.Builder
	toolCalls := make(map[string]*openai.ToolCall)
	argBuilders := make(map[string]*strings.Builder)
	indexToKey := make(map[int]string)
	var firstChunk time.Time
	recvCount := 0

	for {
		select {
		case <-ctx.Done():
			s.debugLogStreamEnd("stream_cancelled", time.Since(start), recvCount, len(toolCalls), ctx.Err())
			events <- NewErrorEvent(ctx.Err())
			return
		default:
			response, err := stream.Recv()
			if err != nil {
				s.debugLogStreamEnd("stream_recv", time.Since(start), recvCount, len(toolCalls), err)
				s.handleStreamEnd(err, &contentBuilder, toolCalls, argBuilders, events)
				return
			}
			recvCount++
			if firstChunk.IsZero() {
				firstChunk = time.Now()
				s.debugLogFirstChunk(time.Since(start))
			}

			if len(response.Choices) == 0 {
				continue
			}

			s.handleStreamChunk(response.Choices[0].Delta, &contentBuilder, toolCalls, argBuilders, indexToKey, events)
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

func (s *Session) handleStreamChunk(delta openai.ChatCompletionStreamChoiceDelta, contentBuilder *strings.Builder, toolCalls map[string]*openai.ToolCall, argBuilders map[string]*strings.Builder, indexToKey map[int]string, events chan<- StreamEvent) {
	if delta.Content != "" {
		contentBuilder.WriteString(delta.Content)
		events <- NewContentEvent(delta.Content)
	}

	for _, tc := range delta.ToolCalls {
		key, entry := accumulateToolCall(toolCalls, argBuilders, indexToKey, tc)
		if entry != nil && key != "" {
			toolCalls[key] = entry
		}
	}
}

func (s *Session) emitToolCalls(finalCalls []openai.ToolCall, events chan<- StreamEvent) {
	for _, call := range finalCalls {
		callCopy := call
		events <- NewToolCallEvent(&callCopy)
	}
}

func (s *Session) debugLogRequest(operation string, req openai.ChatCompletionRequest) {
	if s.Logger == nil {
		return
	}
	s.Logger.Debug().
		Str("operation", operation).
		Str("model", req.Model).
		Int("message_count", len(req.Messages)).
		Int("tool_count", len(req.Tools)).
		Msg("Sending request")
}

func (s *Session) debugLogCompletion(operation string, duration time.Duration, resp openai.ChatCompletionResponse) {
	if s.Logger == nil {
		return
	}
	choiceCount := len(resp.Choices)
	toolCallCount := 0
	if choiceCount > 0 {
		toolCallCount = len(resp.Choices[0].Message.ToolCalls)
	}
	s.Logger.Debug().
		Str("operation", operation).
		Dur("duration_ms", duration).
		Int("choice_count", choiceCount).
		Int("tool_calls", toolCallCount).
		Msg("Received response")
}

func (s *Session) debugLogFirstChunk(elapsed time.Duration) {
	if s.Logger == nil {
		return
	}
	s.Logger.Debug().
		Dur("time_to_first_chunk_ms", elapsed).
		Msg("Received first stream chunk")
}

func (s *Session) debugLogStreamEnd(operation string, duration time.Duration, recvCount, toolCallCount int, err error) {
	if s.Logger == nil {
		return
	}
	event := s.Logger.Debug().
		Str("operation", operation).
		Dur("duration_ms", duration).
		Int("chunks", recvCount).
		Int("tool_call_candidates", toolCallCount)
	if err != nil && err != io.EOF {
		event.Err(err)
	}
	event.Msg("Stream finished")
}

func (s *Session) debugLogError(operation string, err error) {
	if s.Logger == nil {
		return
	}
	s.Logger.Debug().
		Str("operation", operation).
		Err(err).
		Msg("Request failed")
}

// accumulateToolCall merges incremental tool call deltas into a stored call.
func accumulateToolCall(toolCalls map[string]*openai.ToolCall, argBuilders map[string]*strings.Builder, indexToKey map[int]string, tc openai.ToolCall) (string, *openai.ToolCall) {
	key := toolCallKey(tc, indexToKey)
	if key == "" {
		return "", nil
	}
	entry, ok := toolCalls[key]
	if !ok {
		entry = &openai.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: openai.FunctionCall{
				Name: tc.Function.Name,
			},
		}
		if tc.Index != nil {
			entry.Index = tc.Index
		}
		if entry.ID == "" {
			entry.ID = key
		}
	}
	if entry.Function.Name == "" && tc.Function.Name != "" {
		entry.Function.Name = tc.Function.Name
	}
	if entry.Type == "" && tc.Type != "" {
		entry.Type = tc.Type
	}
	if tc.ID != "" {
		entry.ID = tc.ID
	}
	if tc.Index != nil && entry.Index == nil {
		entry.Index = tc.Index
	}

	builder, ok := argBuilders[key]
	if !ok {
		builder = &strings.Builder{}
		argBuilders[key] = builder
	}
	builder.WriteString(tc.Function.Arguments)
	// Don't update Arguments here - wait for finalization to avoid repeated string allocations

	return key, entry
}

func toolCallKey(tc openai.ToolCall, indexToKey map[int]string) string {
	if tc.Index != nil {
		idx := *tc.Index
		if key, ok := indexToKey[idx]; ok {
			return key
		}
		if tc.ID != "" {
			indexToKey[idx] = tc.ID
			return tc.ID
		}
		key := fmt.Sprintf("index:%d", idx)
		indexToKey[idx] = key
		return key
	}
	return tc.ID
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
			args = ""
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
