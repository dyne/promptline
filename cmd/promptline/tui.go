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
	"strings"

	"github.com/chzyer/readline"
	"github.com/rs/zerolog"
	"promptline/internal/chat"
	"promptline/internal/config"
	"promptline/internal/theme"
)

func runTUIMode(logger zerolog.Logger) {
	logger.Debug().Msg("Running in streaming console mode")

	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}

	// Load theme with manager (supports NO_COLOR env var)
	themeMgr, err := theme.NewManager("theme.json")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load theme")
	}
	colors := themeMgr.ColorScheme()

	// Create chat session
	session := chat.NewSession(cfg)
	defer session.Close()
	session.ToolApprover = newToolApprover(colors)

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
		HistoryFile:     cfg.CommandHistoryFile,
		AutoComplete:    getCommandCompleter(),
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
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
	colors.Header.Println("Promptline by Dyne.org")
	fmt.Printf("Connected to: %s\n", session.BaseURL)
	fmt.Printf("Model in use: %s\n", session.Config.Model)
	// fmt.Println("Type /help for commands, Ctrl+C or /quit to exit")
	// fmt.Println("Press Ctrl+R to search conversation history")
	fmt.Println()

	// Track debug mode for commands
	debugMode := false

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
			if handleCommand(line, session, colors, logger, &debugMode) {
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

// getCommandCompleter builds a readline completer from available commands
func getCommandCompleter() *readline.PrefixCompleter {
	commands := getAvailableCommands()
	items := make([]readline.PrefixCompleterInterface, len(commands))
	for i, cmd := range commands {
		items[i] = readline.PcItem("/" + cmd.Name)
	}
	return readline.NewPrefixCompleter(items...)
}
