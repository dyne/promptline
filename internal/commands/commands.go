package commands

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"promptline/internal/chat"
	"promptline/internal/theme"
)

// Handler represents a command handler function
type Handler func(session *chat.Session, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool

// Command represents a TUI command
type Command struct {
	Name        string
	Description string
	Handler     Handler
}

// Registry holds all available commands
type Registry struct {
	commands  map[string]*Command
	DebugMode *bool // Pointer to global debug mode flag
}

// NewRegistry creates a new command registry
func NewRegistry(debugMode *bool) *Registry {
	r := &Registry{
		commands:  make(map[string]*Command),
		DebugMode: debugMode,
	}

	// Register built-in commands
	r.Register("quit", "Exit the application", handleQuit)
	r.Register("exit", "Exit the application", handleQuit)
	r.Register("clear", "Clear conversation history", handleClear)
	r.Register("history", "Display conversation history", handleHistory)
	r.Register("help", "Show available commands", r.handleHelp)
	r.Register("debug", "Toggle debug mode", r.handleDebug)

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
func (r *Registry) Execute(input string, session *chat.Session, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
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
	return cmd.Handler(session, chatView, tuiTheme, app)
}

// GetCommands returns all registered commands
func (r *Registry) GetCommands() map[string]*Command {
	return r.commands
}

// Command handlers

func handleQuit(session *chat.Session, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	app.Stop()
	return true
}

func handleClear(session *chat.Session, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	chatView.Clear()
	session.ClearHistory()
	chatView.SetText(fmt.Sprintf("[%s]Chat history cleared[-]", tuiTheme.ChatSuccessColor))
	return true
}

func handleHistory(session *chat.Session, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	historyText := fmt.Sprintf("[%s]--- Conversation History ---[-]\n", tuiTheme.ChatSuccessColor)
	for _, msg := range session.MessagesSnapshot() {
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
		case "tool":
			role = "Tool"
			color = tuiTheme.ProgressIndicatorColor
		}
		historyText += fmt.Sprintf("[%s]%s:[-] %s\n", color, role, msg.Content)
	}
	historyText += fmt.Sprintf("[%s]--- End History ---[-]", tuiTheme.ChatSuccessColor)
	appendToChat(chatView, historyText)
	chatView.ScrollToEnd()
	return true
}

func (r *Registry) handleHelp(session *chat.Session, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	helpText := fmt.Sprintf("[%s]Available Commands:[-]\n", tuiTheme.ChatSuccessColor)

	for _, cmd := range r.commands {
		helpText += fmt.Sprintf("  [%s]/%s[-] - %s\n", tuiTheme.ChatUserColor, cmd.Name, cmd.Description)
	}

	helpText += fmt.Sprintf("\n[%s]Keyboard Shortcuts:[-]\n", tuiTheme.ChatSuccessColor)
	helpText += fmt.Sprintf("  [%s]Ctrl+Q[-] - Quit application\n", tuiTheme.ChatUserColor)
	helpText += fmt.Sprintf("  [%s]Enter[-] - Send message\n", tuiTheme.ChatUserColor)

	appendToChat(chatView, helpText)
	chatView.ScrollToEnd()
	return true
}

func (r *Registry) handleDebug(session *chat.Session, chatView *tview.TextView, tuiTheme *theme.Theme, app *tview.Application) bool {
	if r.DebugMode != nil {
		*r.DebugMode = !*r.DebugMode
		status := "disabled"
		if *r.DebugMode {
			status = "enabled"
			// Show system prompt in debug mode
			msgs := session.MessagesSnapshot()
			if len(msgs) > 0 {
				appendToChat(chatView, fmt.Sprintf("[%s]System Prompt:[-]\n%s", tuiTheme.ProgressIndicatorColor, msgs[0].Content))
			}
		}
		appendToChat(chatView, fmt.Sprintf("[%s]Debug mode %s[-]", tuiTheme.ChatSuccessColor, status))
	}
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
