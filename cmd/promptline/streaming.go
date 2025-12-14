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

		anySuccess := false
		for _, event := range toolCallsToExecute {
			if executeToolCall(session, event.ToolCall, colors, logger) {
				anySuccess = true
			}
		}

		// Continue conversation with tool results (recursive call) only if at least one succeeded
		if anySuccess {
			fmt.Println()
			colors.Assistant.Print("⟫ ")
			streamConversation(session, "", false, colors, logger)
		} else {
			fmt.Println()
			fmt.Println()
		}
	} else {
		// No tool calls, conversation complete
		fmt.Println() // newline after response
		fmt.Println()
	}
}

// executeToolCall executes a single tool call and adds result to session
// Returns true when the tool succeeds.
func executeToolCall(session *chat.Session, toolCall *openai.ToolCall, colors *theme.ColorScheme, logger zerolog.Logger) bool {
	toolName := toolCall.Function.Name
	toolArgs := toolCall.Function.Arguments

	if toolName == "read_file" && shouldFillPath(toolCall.Function.Arguments) {
		if filled := fillPathFromHistory(session, toolCall, logger); filled != "" {
			toolArgs = filled
		}
	}

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
		return false
	} else {
		logger.Debug().
			Str("tool_name", toolName).
			Int("result_length", len(result.Result)).
			Msg("Tool executed successfully")
	}
	return true
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

func shouldFillPath(args string) bool {
	trimmed := strings.TrimSpace(args)
	return trimmed == "" || trimmed == "{}" || trimmed == "null"
}

func fillPathFromHistory(session *chat.Session, call *openai.ToolCall, logger zerolog.Logger) string {
	lastUser := latestUserMessage(session)
	if lastUser == "" {
		return call.Function.Arguments
	}

	// Pick the first plausible filename/path from the user text.
	candidate := extractPathCandidate(lastUser)
	if candidate == "" {
		return call.Function.Arguments
	}

	// Use the original user-provided form in the arguments. Path validation will still run inside the tool.
	call.Function.Arguments = fmt.Sprintf(`{"path": %q}`, candidate)
	logger.Debug().Str("path", candidate).Msg("Filled missing read_file path from user history")
	return call.Function.Arguments
}

func latestUserMessage(session *chat.Session) string {
	history := session.GetHistory()
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" && strings.TrimSpace(history[i].Content) != "" {
			return history[i].Content
		}
	}
	return ""
}

func extractPathCandidate(message string) string {
	fields := strings.Fields(message)
	for _, f := range fields {
		clean := strings.Trim(f, " \t\n\r\"'`.,;:()[]{}<>")
		if clean == "" {
			continue
		}
		if strings.Contains(clean, "/") || strings.Contains(clean, ".") {
			return clean
		}
	}
	return ""
}
