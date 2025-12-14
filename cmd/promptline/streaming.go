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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/sashabaranov/go-openai"
	"promptline/internal/chat"
	"promptline/internal/theme"
	"promptline/internal/tools"
)

// handleConversation sends user message and streams AI response
func handleConversation(input string, session *chat.Session, colors *theme.ColorScheme, logger zerolog.Logger) {
	logConversation(logger, openai.ChatMessageRoleUser, input)

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
	logConversation(logger, openai.ChatMessageRoleAssistant, responseBuilder.String())

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

// executeToolCall executes a single tool call and adds result to session
func executeToolCall(session *chat.Session, toolCall *openai.ToolCall, colors *theme.ColorScheme, logger zerolog.Logger) {
	toolName := toolCall.Function.Name
	toolArgs := toolCall.Function.Arguments

	logToolCall(logger, toolName, toolArgs)

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
	toolContent := result.Result
	if toolContent == "" && result.Error != nil {
		toolContent = result.Error.Error()
	}
	logConversation(logger, openai.ChatMessageRoleTool, toolContent)

	// Display result to user
	formatted := tools.FormatToolResult(*toolCall, result, true)
	lines := strings.Split(formatted, "\n")
	for _, line := range lines[1:] { // Skip first line (already shown)
		if strings.TrimSpace(line) != "" {
			fmt.Printf("%s\n", line)
		}
	}

	if result.Error != nil {
		logger.Error().
			Str("tool_name", toolName).
			Err(result.Error).
			Msg("Tool execution failed")
	} else {
		logger.Debug().
			Str("tool_name", toolName).
			Int("result_length", len(result.Result)).
			Msg("Tool executed successfully")
	}
}

func logToolCall(logger zerolog.Logger, name, args string) {
	logger.Info().
		Str("role", "tool_call").
		Str("tool", name).
		Str("args", args).
		Msg("conversation")
}

func logConversation(logger zerolog.Logger, role, content string) {
	if content == "" {
		return
	}
	logger.Info().
		Str("role", role).
		Str("content", content).
		Msg("conversation")
}
