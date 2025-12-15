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
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"promptline/internal/chat"
	"promptline/internal/config"
)

func runBatchMode(logger zerolog.Logger) {
	if err := runBatch(logger); err != nil {
		logger.Error().Err(err).Msg("Batch mode failed")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runBatch(logger zerolog.Logger) error {
	logger.Debug().Msg("Running in batch mode")

	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
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
			return fmt.Errorf("failed to get response: %w", err)
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
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}
