package chat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/sashabaranov/go-openai"
	"batchat/internal/config"
)

// Session represents a chat session with context
type Session struct {
	Client    *openai.Client
	Config    *config.Config
	Messages  []openai.ChatCompletionMessage
	Scanner   *bufio.Scanner
	rl        *readline.Instance
	history   []string
}

// NewSession creates a new chat session
func NewSession(cfg *config.Config) *Session {
	// Create client with custom base URL if provided
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
		// For DashScope, we might need to set a custom HTTP client
		clientConfig.HTTPClient = &http.Client{}
	}
	
	client := openai.NewClientWithConfig(clientConfig)

	// Initialize readline instance
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "User: ",
		HistoryFile:     "/tmp/batchat_history",
		AutoComplete:    readline.NewPrefixCompleter(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		panic(err)
	}

	// Initialize with system message
	messages := make([]openai.ChatCompletionMessage, 0)
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: "You are an expert AI assistant helping developers create batch processing jobs using the openbatch Python library. Your role is to help users define their batch tasks, then generate Python code that uses openbatch to efficiently process large datasets at 50% cost. When users want to generate code, provide complete, runnable Python scripts that follow openbatch best practices. Include proper imports, error handling, and comments explaining the code.",
	})

	return &Session{
		Client:   client,
		Config:   cfg,
		Messages: messages,
		Scanner:  bufio.NewScanner(os.Stdin),
		rl:       rl,
		history:  make([]string, 0),
	}
}

// AddMessage adds a message to the conversation history
func (s *Session) AddMessage(role, content string) {
	s.Messages = append(s.Messages, openai.ChatCompletionMessage{
		Role:    role,
		Content: content,
	})
}

// GetResponse gets a response from the OpenAI API
func (s *Session) GetResponse(prompt string) (string, error) {
	// Add user message to history
	s.AddMessage(openai.ChatMessageRoleUser, prompt)

	// Prepare the request
	req := openai.ChatCompletionRequest{
		Model: s.Config.Model,
		Messages: s.Messages,
	}

	// Add optional parameters if they exist in config
	if s.Config.Temperature != nil {
		req.Temperature = *s.Config.Temperature
	}
	
	if s.Config.MaxTokens != nil {
		req.MaxTokens = *s.Config.MaxTokens
	}

	// Get response from OpenAI
	resp, err := s.Client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return "", err
	}

	// Add assistant response to history
	responseText := resp.Choices[0].Message.Content
	s.AddMessage(openai.ChatMessageRoleAssistant, responseText)

	return responseText, nil
}

// GetStreamingResponse gets a streaming response from the OpenAI API
func (s *Session) GetStreamingResponse(prompt string) error {
	// Add user message to history
	s.AddMessage(openai.ChatMessageRoleUser, prompt)

	// Prepare the request
	req := openai.ChatCompletionRequest{
		Model: s.Config.Model,
		Messages: s.Messages,
		Stream: true,
	}

	// Add optional parameters if they exist in config
	if s.Config.Temperature != nil {
		req.Temperature = *s.Config.Temperature
	}
	
	if s.Config.MaxTokens != nil {
		req.MaxTokens = *s.Config.MaxTokens
	}

	// Get streaming response from OpenAI
	stream, err := s.Client.CreateChatCompletionStream(context.Background(), req)
	if err != nil {
		return err
	}
	defer stream.Close()

	// Process the stream
	fullResponse := ""
	fmt.Print("Assistant: ")
	for {
		response, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
			content := response.Choices[0].Delta.Content
			fmt.Print(content)
			fullResponse += content
		}
	}
	fmt.Println() // New line after response

	// Add assistant response to history
	s.AddMessage(openai.ChatMessageRoleAssistant, fullResponse)
	
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
		Model: s.Config.Model,
		Messages: s.Messages,
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
	// Keep the system message but clear the rest
	systemMsg := s.Messages[0]
	s.Messages = []openai.ChatCompletionMessage{systemMsg}
}

// GetInput gets input from the user
func (s *Session) GetInput() (string, error) {
	line, err := s.rl.Readline()
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
		s.rl.SaveHistory(line)
	}

	return line, nil
}

// PrintHistory prints the conversation history
func (s *Session) PrintHistory() {
	fmt.Println("--- Conversation History ---")
	for _, msg := range s.Messages {
		role := "Unknown"
		switch msg.Role {
		case openai.ChatMessageRoleSystem:
			role = "System"
		case openai.ChatMessageRoleUser:
			role = "User"
		case openai.ChatMessageRoleAssistant:
			role = "Assistant"
		}
		fmt.Printf("%s: %s\n", role, msg.Content)
	}
	fmt.Println("--- End History ---")
}

// Close closes the readline instance
func (s *Session) Close() error {
	if s.rl != nil {
		return s.rl.Close()
	}
	return nil
}