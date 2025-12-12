package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/chzyer/readline"
	"github.com/pterm/pterm"
	"github.com/rs/zerolog"
	"github.com/sashabaranov/go-openai"
	"promptline/internal/chat"
	"promptline/internal/config"
	"promptline/internal/theme"
)

var (
	debugMode = flag.Bool("d", false, "Enable debug mode")
	logFile   = flag.String("log-file", "", "Log file path (logs disabled by default)")
)

func main() {
	flag.Parse()

	// Initialize logger
	logger := initLogger(*debugMode, *logFile)
	logger.Info().Msg("Promptline starting")

	// Check if we're running in batch mode (with "-" argument)
	args := flag.Args()
	if len(args) > 0 && args[0] == "-" {
		runBatchMode(logger)
		return
	}

	// Run in normal TUI mode
	runTUIMode(logger)
}

func initLogger(debug bool, logFilePath string) zerolog.Logger {
	// Set log level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// Configure output
	var output io.Writer
	if logFilePath != "" {
		// Log to file only
		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		output = file
	} else {
		// No logging to console by default - use io.Discard
		output = io.Discard
	}

	// Create logger with timestamp
	return zerolog.New(output).With().Timestamp().Logger()
}

func runBatchMode(logger zerolog.Logger) {
	logger.Debug().Msg("Running in batch mode")

	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}

	// Create chat session
	session := chat.NewSession(cfg)
	defer session.Close()

	// Load conversation history
	if cfg.HistoryFile != "" {
		if err := session.LoadConversationHistory(cfg.HistoryFile, cfg.HistoryMaxMessages); err != nil {
			logger.Warn().Err(err).Msg("Failed to load conversation history")
		} else {
			historyCount := len(session.GetHistory())
			if historyCount > 0 {
				logger.Debug().Int("messages", historyCount).Msg("Loaded conversation history")
			}
		}
	}

	// Read input from stdin
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := scanner.Text()
		logger.Info().Str("user_input", input).Msg("User input received")

		// Get response from AI
		start := time.Now()
		response, err := session.GetResponse(input)
		duration := time.Since(start)

		if err != nil {
			logger.Error().Err(err).Dur("duration_ms", duration).Msg("Error getting response")
			log.Fatalf("Error getting response: %v", err)
		}

		logger.Info().
			Str("model_response", response).
			Dur("duration_ms", duration).
			Msg("AI response received")

		// Output response
		fmt.Println(response)
		
		// Save conversation history
		if cfg.HistoryFile != "" {
			if err := session.SaveConversationHistory(cfg.HistoryFile); err != nil {
				logger.Warn().Err(err).Msg("Failed to save conversation history")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Fatal().Err(err).Msg("Error reading input")
	}
}

func runTUIMode(logger zerolog.Logger) {
	logger.Debug().Msg("Running in streaming console mode")

	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}

	// Load theme and convert to color scheme
	tuiTheme, err := theme.LoadTheme("theme.json")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load theme")
	}
	colors := tuiTheme.ToColorScheme()

	// Create chat session
	session := chat.NewSession(cfg)
	defer session.Close()

	// Load conversation history
	if cfg.HistoryFile != "" {
		if err := session.LoadConversationHistory(cfg.HistoryFile, cfg.HistoryMaxMessages); err != nil {
			logger.Warn().Err(err).Msg("Failed to load conversation history")
		} else {
			historyCount := len(session.GetHistory())
			if historyCount > 0 {
				logger.Debug().Int("messages", historyCount).Msg("Loaded conversation history")
			}
		}
	}

	// Initialize readline with dynamic command completion and Ctrl-R handler
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          colors.User.Sprint("❯ "),
		HistoryFile:     ".promptline_history",
		AutoComplete:    getCommandCompleter(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		// Custom keybinding for Ctrl-R
		FuncOnWidthChanged: func(f func()) {
			// Not used but required for custom operations
		},
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize readline")
	}
	defer rl.Close()
	
	// Set up Ctrl-R handler for history search
	rl.Config.FuncFilterInputRune = func(r rune) (rune, bool) {
		if r == 18 { // Ctrl-R
			// Trigger history search
			selected := searchConversationHistory(session, colors, logger)
			if selected != "" {
				// Write the selected text to readline buffer
				rl.WriteStdin([]byte(selected))
			}
			return 0, false
		}
		return r, true
	}

	// Display header
	colors.Header.Println("Promptline - AI Chat")
	fmt.Println("Type /help for commands, Ctrl+C or /quit to exit")
	fmt.Println("Press Ctrl+R to search conversation history")
	fmt.Println()

	// Main event loop
	for {
		line, err := rl.Readline()
		if err != nil {
			// EOF or interrupt
			logger.Debug().Msg("Readline interrupted")
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		logger.Info().Str("user_input", line).Msg("User input received")

		// Handle slash commands
		if strings.HasPrefix(line, "/") {
			if handleCommand(line, session, colors, logger) {
				// /quit was called
				break
			}
			continue
		}

		// Handle conversation
		handleConversation(line, session, colors, logger)
		
		// Save conversation history after each turn
		if cfg.HistoryFile != "" {
			if err := session.SaveConversationHistory(cfg.HistoryFile); err != nil {
				logger.Warn().Err(err).Msg("Failed to save conversation history")
			}
		}
	}

	logger.Info().Msg("Session ended")
}

// Command represents a slash command
type Command struct {
	Name        string
	Description string
}

// getAvailableCommands returns the list of all slash commands
func getAvailableCommands() []Command {
	return []Command{
		{Name: "help", Description: "Show available commands"},
		{Name: "clear", Description: "Clear conversation history"},
		{Name: "history", Description: "Display conversation history"},
		{Name: "debug", Description: "Toggle debug mode"},
		{Name: "permissions", Description: "Show and adjust tool permissions"},
		{Name: "quit", Description: "Exit the application"},
		{Name: "exit", Description: "Exit the application"},
	}
}

// getCommandCompleter builds a readline completer from available commands
func getCommandCompleter() *readline.PrefixCompleter {
	commands := getAvailableCommands()
	items := make([]readline.PrefixCompleterInterface, len(commands))
	for i, cmd := range commands {
		items[i] = readline.PcItem("/" + cmd.Name)
	}
	return readline.NewPrefixCompleter(items...)
}

// handleCommand processes slash commands, returns true if should quit
func handleCommand(input string, session *chat.Session, colors *theme.ColorScheme, logger zerolog.Logger) bool {
	cmdName := strings.TrimPrefix(input, "/")
	cmdName = strings.ToLower(strings.TrimSpace(cmdName))

	logger.Debug().Str("command", cmdName).Msg("Executing command")

	// Execute command based on name
	switch cmdName {
	case "help":
		colors.Header.Println("\nAvailable Commands:")
		seen := make(map[string]bool)
		for _, cmd := range getAvailableCommands() {
			if seen[cmd.Name] {
				continue
			}
			seen[cmd.Name] = true
			fmt.Printf("  /%-12s - %s\n", cmd.Name, cmd.Description)
		}
		fmt.Println("\nKeyboard Shortcuts:")
		fmt.Println("  Ctrl+R       - Search conversation history (fuzzy search)")
		fmt.Println("  Ctrl+↑/↓     - Navigate command history")
		fmt.Println("  Tab          - Auto-complete commands")
		fmt.Println()
		return false
		
	case "clear":
		session.ClearHistory()
		colors.Success.Println("✓ Conversation history cleared")
		return false
		
	case "history":
		showHistory(session, colors)
		return false
		
	case "debug":
		*debugMode = !*debugMode
		if *debugMode {
			colors.Success.Println("✓ Debug mode enabled")
		} else {
			colors.Success.Println("✓ Debug mode disabled")
		}
		return false
		
	case "permissions":
		showPermissions(session, colors)
		return false
		
	case "quit", "exit":
		return true
		
	default:
		colors.Error.Printf("✗ Unknown command: /%s (type /help for available commands)\n", cmdName)
		return false
	}
}

func showHistory(session *chat.Session, colors *theme.ColorScheme) {
	messages := session.GetHistory()
	if len(messages) == 0 {
		colors.Error.Println("No conversation history")
		return
	}

	colors.Header.Println("\nConversation History:")
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			colors.User.Print("❯ ")
			fmt.Printf("%s\n", msg.Content)
		case "assistant":
			colors.Assistant.Print("⟫ ")
			fmt.Printf("%s\n", msg.Content)
		case "system":
			fmt.Printf("[System] %s\n", msg.Content)
		}
	}
	fmt.Println()
}

func showPermissions(session *chat.Session, colors *theme.ColorScheme) {
	colors.Header.Println("\nTool Permissions:")
	
	toolNames := session.ToolRegistry.GetToolNames()
	if len(toolNames) == 0 {
		fmt.Println("No tools available")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(w, "Tool\tAllowed\tRequire Confirmation")
	fmt.Fprintln(w, "────\t───────\t────────────────────")
	
	for _, name := range toolNames {
		perm := session.ToolRegistry.GetPermission(name)
		allowed := "✓"
		if !perm.Allowed {
			allowed = "✗"
		}
		confirm := "No"
		if perm.RequireConfirmation {
			confirm = "Yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, allowed, confirm)
	}
	w.Flush()
	fmt.Println()
}

// handleConversation sends user message and streams AI response
func handleConversation(input string, session *chat.Session, colors *theme.ColorScheme, logger zerolog.Logger) {
	// Log user input (already echoed by readline, don't print again)
	logger.Info().Str("user_input", input).Msg("User input received")

	// Stream the conversation, handling tool calls recursively
	streamConversation(session, input, true, colors, logger)
}

// streamConversation handles streaming with tool execution
func streamConversation(session *chat.Session, input string, includeUserMessage bool, colors *theme.ColorScheme, logger zerolog.Logger) {
	// Create streaming events channel
	events := make(chan chat.StreamEvent, 10)
	ctx := context.Background()

	// Start streaming in goroutine
	go session.StreamResponseWithContext(ctx, input, includeUserMessage, events)

	// Display assistant prefix with special character (only for new conversations)
	if includeUserMessage {
		colors.Assistant.Print("⟫ ")
	}

	start := time.Now()
	var responseBuilder strings.Builder
	var toolCallsToExecute []*chat.StreamEvent

	// Process streaming events
	for event := range events {
		switch event.Type {
		case chat.StreamEventContent:
			// Print content chunk directly
			fmt.Print(event.Content)
			responseBuilder.WriteString(event.Content)

		case chat.StreamEventToolCall:
			// Collect tool calls for execution after stream completes
			if event.ToolCall != nil {
				eventCopy := event
				toolCallsToExecute = append(toolCallsToExecute, &eventCopy)
			}

		case chat.StreamEventError:
			colors.Error.Printf("\n✗ Error: %v\n", event.Err)
			logger.Error().Err(event.Err).Msg("Streaming error")
			return
		}
	}

	duration := time.Since(start)
	
	// Log the response
	logger.Info().
		Str("model_response", responseBuilder.String()).
		Dur("duration_ms", duration).
		Int("tool_calls", len(toolCallsToExecute)).
		Msg("AI response received")

	// Execute any tool calls and continue conversation
	if len(toolCallsToExecute) > 0 {
		fmt.Println() // newline before tool execution
		
		for _, event := range toolCallsToExecute {
			executeToolCall(session, event.ToolCall, colors, logger)
		}
		
		// Continue conversation with tool results (recursive call)
		fmt.Println()
		colors.Assistant.Print("⟫ ")
		streamConversation(session, "", false, colors, logger)
	} else {
		// No tool calls, conversation complete
		fmt.Println() // newline after response
		fmt.Println()
	}
}

// searchConversationHistory shows an interactive fuzzy search of conversation history
func searchConversationHistory(session *chat.Session, colors *theme.ColorScheme, logger zerolog.Logger) string {
	history := session.GetHistory()
	if len(history) == 0 {
		colors.Error.Println("\nNo conversation history available")
		return ""
	}

	// Build list of user messages only
	var userMessages []string
	for _, msg := range history {
		if msg.Role == "user" && msg.Content != "" {
			userMessages = append(userMessages, msg.Content)
		}
	}

	if len(userMessages) == 0 {
		colors.Error.Println("\nNo user messages in history")
		return ""
	}

	// Deduplicate and reverse (most recent first)
	seen := make(map[string]bool)
	var uniqueMessages []string
	for i := len(userMessages) - 1; i >= 0; i-- {
		msg := userMessages[i]
		if !seen[msg] {
			seen[msg] = true
			uniqueMessages = append(uniqueMessages, msg)
		}
	}

	if len(uniqueMessages) == 0 {
		return ""
	}

	// Show interactive selector
	fmt.Println() // newline before selector
	colors.Header.Println("Search History (Ctrl-C to cancel, arrows to navigate):")
	
	selected, err := pterm.DefaultInteractiveSelect.
		WithOptions(uniqueMessages).
		WithDefaultText("Select a previous prompt").
		WithFilter(true). // Enable fuzzy search
		Show()
	
	if err != nil {
		logger.Debug().Err(err).Msg("History search cancelled")
		return ""
	}

	return selected
}

// executeToolCall executes a single tool call and adds result to session
func executeToolCall(session *chat.Session, toolCall *openai.ToolCall, colors *theme.ColorScheme, logger zerolog.Logger) {
	toolName := toolCall.Function.Name
	toolArgs := toolCall.Function.Arguments
	
	// Show what tool is being called
	colors.ProgressIndicator.Printf("🔧 [%s]", toolName)
	fmt.Println()
	
	logger.Debug().
		Str("tool_name", toolName).
		Str("tool_args", toolArgs).
		Msg("Executing tool")
	
	// Execute the tool
	result := session.ToolRegistry.ExecuteOpenAIToolCall(*toolCall)
	
	// Add result to conversation history
	session.AddToolResultMessage(*toolCall, result)
	
	// Display result to user
	if result.Error != nil {
		colors.Error.Printf("   ✗ Error: %v\n", result.Error)
		logger.Error().
			Str("tool_name", toolName).
			Err(result.Error).
			Msg("Tool execution failed")
	} else {
		// Truncate long results for display
		displayResult := result.Result
		if len(displayResult) > 200 {
			displayResult = displayResult[:200] + "..."
		}
		fmt.Printf("   ✓ Result: %s\n", displayResult)
		
		logger.Debug().
			Str("tool_name", toolName).
			Int("result_length", len(result.Result)).
			Msg("Tool executed successfully")
	}
}
