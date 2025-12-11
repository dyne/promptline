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
	"github.com/rs/zerolog"
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

	// Initialize readline with special prompt character
	rl, err := readline.New(colors.User.Sprint("❯ "))
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize readline")
	}
	defer rl.Close()

	// Display header
	colors.Header.Println("Promptline - AI Chat")
	fmt.Println("Type /help for commands, Ctrl+C or /quit to exit")
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
	}

	logger.Info().Msg("Session ended")
}

// handleCommand processes slash commands, returns true if should quit
func handleCommand(input string, session *chat.Session, colors *theme.ColorScheme, logger zerolog.Logger) bool {
	cmdName := strings.TrimPrefix(input, "/")
	cmdName = strings.ToLower(strings.TrimSpace(cmdName))

	logger.Debug().Str("command", cmdName).Msg("Executing command")

	switch cmdName {
	case "quit", "exit":
		return true
	case "help":
		showHelp(colors)
	case "clear":
		session.ClearHistory()
		colors.Success.Println("✓ Conversation history cleared")
	case "history":
		showHistory(session, colors)
	case "debug":
		*debugMode = !*debugMode
		if *debugMode {
			colors.Success.Println("✓ Debug mode enabled")
		} else {
			colors.Success.Println("✓ Debug mode disabled")
		}
	case "permissions":
		showPermissions(session, colors)
	default:
		colors.Error.Printf("✗ Unknown command: /%s (type /help for available commands)\n", cmdName)
	}

	return false
}

func showHelp(colors *theme.ColorScheme) {
	colors.Header.Println("\nAvailable Commands:")
	fmt.Println("  /help        - Show this help message")
	fmt.Println("  /clear       - Clear conversation history")
	fmt.Println("  /history     - Display conversation history")
	fmt.Println("  /debug       - Toggle debug mode")
	fmt.Println("  /permissions - Show and adjust tool permissions")
	fmt.Println("  /quit        - Exit the application")
	fmt.Println()
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

	// Create streaming events channel
	events := make(chan chat.StreamEvent, 10)
	ctx := context.Background()

	// Start streaming in goroutine
	go session.StreamResponseWithContext(ctx, input, true, events)

	// Display assistant prefix with special character
	colors.Assistant.Print("⟫ ")

	start := time.Now()
	var responseBuilder strings.Builder

	// Process streaming events
	for event := range events {
		switch event.Type {
		case chat.StreamEventContent:
			// Print content chunk directly
			fmt.Print(event.Content)
			responseBuilder.WriteString(event.Content)

		case chat.StreamEventToolCall:
			// Show tool call
			if event.ToolCall != nil {
				toolName := event.ToolCall.Function.Name
				toolArgs := event.ToolCall.Function.Arguments
				
				fmt.Println()
				colors.ProgressIndicator.Printf("[Tool Call] %s", toolName)
				fmt.Println()
				
				logger.Debug().
					Str("tool_name", toolName).
					Str("tool_args", toolArgs).
					Msg("Tool call requested")
				
				// Execute tool (simplified for now)
				fmt.Printf("  Arguments: %s\n", toolArgs)
			}

		case chat.StreamEventError:
			colors.Error.Printf("\n✗ Error: %v\n", event.Err)
			logger.Error().Err(event.Err).Msg("Streaming error")
			return
		}
	}

	duration := time.Since(start)
	fmt.Println() // newline after response
	fmt.Println()

	logger.Info().
		Str("model_response", responseBuilder.String()).
		Dur("duration_ms", duration).
		Msg("AI response received")
}
