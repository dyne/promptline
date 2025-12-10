package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/alpkeskin/gotoon"
	"github.com/chzyer/readline"
	"github.com/sashabaranov/go-openai"
	"promptline/internal/config"
	"promptline/internal/tools"
)

// Session represents a chat session with context
type Session struct {
	Client       *openai.Client
	Config       *config.Config
	Messages     []openai.ChatCompletionMessage
	Scanner      *bufio.Scanner
	RL           *readline.Instance
	history      []string
	ToolRegistry *tools.Registry
	mu           sync.Mutex
}

// NewSession creates a new chat session
func NewSession(cfg *config.Config) *Session {
	// Create client with custom base URL if provided
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.APIURL != "" {
		clientConfig.BaseURL = cfg.APIURL
		// For DashScope, we might need to set a custom HTTP client
		clientConfig.HTTPClient = &http.Client{}
	}

	client := openai.NewClientWithConfig(clientConfig)

	// Initialize readline instance
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "User: ",
		HistoryFile:     ".promptline_history",
		AutoComplete:    readline.NewPrefixCompleter(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}

	// Initialize tool registry
	toolRegistry := tools.NewRegistry()

	systemPrompt := "You are an expert AI assistant to help software development. You will use bd (beads) for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods. Use available tools via function calls when they are relevant.\n"
	systemPrompt += "Tool outputs must be formatted using TOON (Token-Oriented Object Notation). Return concise TOON blocks for tool results/errors. Keep function.arguments strictly valid JSON when requesting tools. Do not wrap TOON in markdown fences.\n"

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
		Scanner:      bufio.NewScanner(os.Stdin),
		RL:           rl,
		history:      make([]string, 0),
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
	payload := struct {
		Result string `json:"result,omitempty"`
		Error  string `json:"error,omitempty"`
	}{
		Result: result.Result,
	}
	if result.Error != nil {
		payload.Error = result.Error.Error()
	}
	content := result.Result
	if encoded, err := gotoon.Encode(payload); err == nil {
		content = encoded
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

// GetResponse gets a response from the OpenAI API
func (s *Session) GetResponseWithContext(ctx context.Context, prompt string) (string, error) {
	s.AddMessage(openai.ChatMessageRoleUser, prompt)

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
		return "", err
	}

	response := resp.Choices[0].Message
	s.AddAssistantMessage(response.Content, response.ToolCalls)
	return response.Content, nil
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

// StreamResponseWithContext gets a streaming response from the OpenAI API and sends it through a channel of events.
// If includeUserMessage is true, the prompt is added as a user message before sending the request.
func (s *Session) StreamResponseWithContext(ctx context.Context, prompt string, includeUserMessage bool, events chan<- StreamEvent) {
	defer close(events)

	if includeUserMessage && prompt != "" {
		s.AddMessage(openai.ChatMessageRoleUser, prompt)
	}

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

	stream, err := s.Client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		events <- StreamEvent{Type: StreamEventError, Err: err}
		return
	}
	defer stream.Close()

	var contentBuilder strings.Builder
	toolCalls := make(map[string]*openai.ToolCall)
	argBuilders := make(map[string]*strings.Builder)

	for {
		select {
		case <-ctx.Done():
			events <- StreamEvent{Type: StreamEventError, Err: ctx.Err()}
			return
		default:
			response, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					// Persist assistant message (with tool calls if any)
					finalCalls := make([]openai.ToolCall, 0, len(toolCalls))
					for _, call := range toolCalls {
						// ensure final arguments are up to date from builder map
						if builder, ok := argBuilders[call.ID]; ok {
							call.Function.Arguments = builder.String()
						}
						if call.Function.Name == "" {
							call.Function.Name = "unknown_tool"
						}
						finalCalls = append(finalCalls, *call)
					}
					s.AddAssistantMessage(contentBuilder.String(), finalCalls)

					// Emit tool calls so the caller can execute them
					for _, call := range finalCalls {
						callCopy := call
						events <- StreamEvent{Type: StreamEventToolCall, ToolCall: &callCopy}
					}
					return
				}
				events <- StreamEvent{Type: StreamEventError, Err: err}
				return
			}

			if len(response.Choices) == 0 {
				continue
			}

			delta := response.Choices[0].Delta
			if delta.Content != "" {
				content := delta.Content
				contentBuilder.WriteString(content)
				events <- StreamEvent{Type: StreamEventContent, Content: content}
			}

			for _, tc := range delta.ToolCalls {
				entry := accumulateToolCall(toolCalls, argBuilders, tc)
				if entry != nil {
					toolCalls[tc.ID] = entry
				}
			}
		}
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
	entry.Function.Arguments = builder.String()

	return entry
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

// GeneratePythonCode generates Python code for batch processing using openbatch
func (s *Session) GeneratePythonCode() error {
	fmt.Println("Generating Python code for your batch task...")
	fmt.Println("Please describe what kind of batch processing you need:")

	// Get specific input for code generation
	input, err := s.GetInput()
	if err != nil {
		return err
	}

	// Add a specific prompt to guide the AI to generate Python code
	codePrompt := fmt.Sprintf("Generate a complete Python script using the openbatch library for the following task: %s\n\n"+
		"Requirements:\n"+
		"1. Include all necessary imports\n"+
		"2. Use proper error handling\n"+
		"3. Follow openbatch best practices\n"+
		"4. Include comments explaining the code\n"+
		"5. Make it runnable as a standalone script\n"+
		"6. Assume openbatch is installed (pip install openbatch)\n"+
		"7. Output the code in a single Python file with no additional text", input)

	// Add user message to history
	s.AddMessage(openai.ChatMessageRoleUser, codePrompt)

	// Prepare the request specifically for code generation
	req := openai.ChatCompletionRequest{
		Model:    s.Config.Model,
		Messages: s.MessagesSnapshot(),
	}

	// Add optional parameters if they exist in config
	if s.Config.Temperature != nil {
		req.Temperature = *s.Config.Temperature
	}

	if s.Config.MaxTokens != nil {
		req.MaxTokens = *s.Config.MaxTokens
	} else {
		// For code generation, we might need more tokens
		req.MaxTokens = 2000
	}

	// Get response from OpenAI
	resp, err := s.Client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return err
	}

	// Add assistant response to history
	responseText := resp.Choices[0].Message.Content
	s.AddMessage(openai.ChatMessageRoleAssistant, responseText)

	fmt.Println("Generated Python code:")
	fmt.Println("======================")
	fmt.Println(responseText)
	fmt.Println("======================")
	fmt.Println("Save this code to a .py file and run it with Python after installing openbatch:")
	fmt.Println("pip install openbatch")
	fmt.Println("python your_script.py")

	return nil
}

// ClearHistory clears the conversation history
func (s *Session) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	systemMsg := s.Messages[0]
	s.Messages = []openai.ChatCompletionMessage{systemMsg}
}

// GetInput gets input from the user
func (s *Session) GetInput() (string, error) {
	line, err := s.RL.Readline()
	if err != nil {
		if err == readline.ErrInterrupt {
			return "", err
		} else if err == io.EOF {
			return "exit", nil
		}
		return "", err
	}

	// Add to history if not empty
	line = strings.TrimSpace(line)
	if line != "" {
		s.history = append(s.history, line)
		s.RL.SaveHistory(line)
	}

	return line, nil
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
func (s *Session) FormatToolCallDisplay(toolCall openai.ToolCall, result *tools.ToolResult) string {
	var argsStr string
	if toolCall.Function.Arguments != "" {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil && len(args) > 0 {
			parts := make([]string, 0, len(args))
			for key, value := range args {
				parts = append(parts, fmt.Sprintf("%s=%v", key, value))
			}
			argsStr = strings.Join(parts, ", ")
		} else {
			argsStr = toolCall.Function.Arguments
		}
	}

	var sb strings.Builder
	if argsStr != "" {
		sb.WriteString(fmt.Sprintf("🔧 Executed: %s(%s)\n", toolCall.Function.Name, argsStr))
	} else {
		sb.WriteString(fmt.Sprintf("🔧 Executed: %s()\n", toolCall.Function.Name))
	}

	if result.Error != nil {
		sb.WriteString(fmt.Sprintf("❌ Error: %v\n", result.Error))
	} else {
		sb.WriteString(fmt.Sprintf("✓ Result:\n%s\n", result.Result))
	}
	return sb.String()
}

// Close closes the readline instance
func (s *Session) Close() error {
	if s.RL != nil {
		return s.RL.Close()
	}
	return nil
}
