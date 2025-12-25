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

package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/pterm/pterm"
	"github.com/rs/zerolog"
	"promptline/internal/chat"
	"promptline/internal/theme"
	"promptline/internal/tools"
)

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

// handleCommand processes slash commands, returns true if should quit
func handleCommand(input string, session *chat.Session, colors *theme.ColorScheme, logger zerolog.Logger, debugMode *bool) bool {
	cmdName := strings.TrimPrefix(input, "/")
	cmdName = strings.ToLower(strings.TrimSpace(cmdName))

	logger.Debug().Str("command", cmdName).Msg("Executing command")

	// Execute command based on name
	switch cmdName {
	case "help":
		showHelp(colors)
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

func showHelp(colors *theme.ColorScheme) {
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
	fmt.Fprintln(w, "Tool\tPermission")
	fmt.Fprintln(w, "────\t──────────")

	for _, name := range toolNames {
		perm := session.ToolRegistry.GetPermission(name)
		level := string(perm.Level)
		if level == "" {
			level = string(tools.PermissionAsk)
		}
		fmt.Fprintf(w, "%s\t%s\n", name, level)
	}
	w.Flush()
	fmt.Println()
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
