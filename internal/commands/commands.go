package commands

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
	"github.com/sashabaranov/go-openai"

	"batchat/internal/theme"
)

// Handler represents a command handler function
type Handler func(messages *[]openai.ChatCompletionMessage, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool

// Command represents a TUI command
type Command struct {
	Name        string
	Description string
	Handler     Handler
}

// Registry holds all available commands
type Registry struct {
	commands map[string]*Command
}

// NewRegistry creates a new command registry
func NewRegistry() *Registry {
	r := &Registry{
		commands: make(map[string]*Command),
	}
	
	// Register built-in commands
	r.Register("quit", "Exit the application", handleQuit)
	r.Register("exit", "Exit the application", handleQuit)
	r.Register("clear", "Clear conversation history", handleClear)
	r.Register("history", "Display conversation history", handleHistory)
	r.Register("generate", "Generate Python code for batch processing", handleGenerate)
	r.Register("help", "Show available commands", handleHelp)
	
	return r
}

// Register adds a new command to the registry
func (r *Registry) Register(name, description string, handler Handler) {
	r.commands[name] = &Command{
		Name:        name,
		Description: description,
		Handler:     handler,
	}
}

// Execute runs a command if it exists
func (r *Registry) Execute(input string, messages *[]openai.ChatCompletionMessage, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	// Check if input starts with /
	if !strings.HasPrefix(input, "/") {
		return false
	}
	
	// Extract command name
	cmdName := strings.TrimPrefix(input, "/")
	cmdName = strings.ToLower(strings.TrimSpace(cmdName))
	
	// Look up command
	cmd, exists := r.commands[cmdName]
	if !exists {
		appendToChat(chatView, fmt.Sprintf("[%s]Unknown command: /%s (type /help for available commands)[-]", tuiTheme.ChatErrorColor, cmdName))
		return true
	}
	
	// Execute command
	return cmd.Handler(messages, chatView, tuiTheme, app)
}

// GetCommands returns all registered commands
func (r *Registry) GetCommands() map[string]*Command {
	return r.commands
}

// Command handlers

func handleQuit(messages *[]openai.ChatCompletionMessage, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	app.Stop()
	return true
}

func handleClear(messages *[]openai.ChatCompletionMessage, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	chatView.Clear()
	// Keep the system message but clear the rest
	if len(*messages) > 0 {
		systemMsg := (*messages)[0]
		*messages = []openai.ChatCompletionMessage{systemMsg}
	}
	chatView.SetText(fmt.Sprintf("[%s]Chat history cleared[-]", tuiTheme.ChatSuccessColor))
	return true
}

func handleHistory(messages *[]openai.ChatCompletionMessage, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	historyText := fmt.Sprintf("[%s]--- Conversation History ---[-]\n", tuiTheme.ChatSuccessColor)
	for _, msg := range *messages {
		role := "Unknown"
		color := tuiTheme.ChatAssistantColor
		switch msg.Role {
		case "system":
			role = "System"
			color = tuiTheme.ProgressIndicatorColor
		case "user":
			role = "User"
			color = tuiTheme.ChatUserColor
		case "assistant":
			role = "Assistant"
			color = tuiTheme.ChatAssistantColor
		}
		historyText += fmt.Sprintf("[%s]%s:[-] %s\n", color, role, msg.Content)
	}
	historyText += fmt.Sprintf("[%s]--- End History ---[-]", tuiTheme.ChatSuccessColor)
	appendToChat(chatView, historyText)
	chatView.ScrollToEnd()
	return true
}

func handleGenerate(messages *[]openai.ChatCompletionMessage, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	appendToChat(chatView, fmt.Sprintf("[%s]Code generation feature coming soon - ask the AI to generate Python code using openbatch[-]", tuiTheme.ProgressIndicatorColor))
	return true
}

func handleHelp(messages *[]openai.ChatCompletionMessage, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	helpText := fmt.Sprintf("[%s]Available Commands:[-]\n", tuiTheme.ChatSuccessColor)
	
	// Get all commands and display them
	commands := []string{"help", "history", "clear", "generate", "quit", "exit"}
	registry := NewRegistry()
	for _, cmdName := range commands {
		if cmd, exists := registry.commands[cmdName]; exists {
			helpText += fmt.Sprintf("  [%s]/%s[-] - %s\n", tuiTheme.ChatUserColor, cmd.Name, cmd.Description)
		}
	}
	
	helpText += fmt.Sprintf("\n[%s]Keyboard Shortcuts:[-]\n", tuiTheme.ChatSuccessColor)
	helpText += fmt.Sprintf("  [%s]Ctrl+Q[-] - Quit application\n", tuiTheme.ChatUserColor)
	helpText += fmt.Sprintf("  [%s]Enter[-] - Send message\n", tuiTheme.ChatUserColor)
	
	appendToChat(chatView, helpText)
	chatView.ScrollToEnd()
	return true
}

// Helper function to append text to chat view
func appendToChat(chatView *tview.TextView, text string) {
	currentText := chatView.GetText(false)
	if currentText != "" {
		currentText += "\n"
	}
	chatView.SetText(currentText + text)
}
