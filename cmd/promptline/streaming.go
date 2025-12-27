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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/sashabaranov/go-openai"
	"promptline/internal/chat"
	"promptline/internal/tools"
)

// handleConversation sends user message and streams AI response
func handleConversation(input string, session *chat.Session, logger zerolog.Logger, canceler *operationCanceler) {
	sessionLogger := logger.With().Str("session_id", session.SessionID).Logger()
	logConversation(sessionLogger, openai.ChatMessageRoleUser, input)

	// Stream the conversation, handling tool calls recursively
	streamConversation(session, input, true, sessionLogger, canceler)
}

// streamConversation handles streaming with tool execution
func streamConversation(session *chat.Session, input string, includeUserMessage bool, logger zerolog.Logger, canceler *operationCanceler) {
	sessionLogger := logger.With().Str("session_id", session.SessionID).Logger()
	// Create streaming events channel
	events := make(chan chat.StreamEvent, 10)
	ctx, cancel := context.WithCancel(context.Background())
	if canceler != nil {
		canceler.Set(cancel)
	}
	defer func() {
		cancel()
		if canceler != nil {
			canceler.Clear()
		}
	}()

	// Start streaming in goroutine
	go session.StreamResponseWithContext(ctx, input, includeUserMessage, events)

	// Display assistant prefix with special character (only for new conversations)
	if includeUserMessage {
		fmt.Print("⟫ ")
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
			if errors.Is(event.Err, context.Canceled) {
				fmt.Println("\n⟫ cancelled")
				sessionLogger.Debug().Err(event.Err).Msg("Streaming cancelled")
				return
			}
			fmt.Printf("\n✗ Error: %v\n", event.Err)
			sessionLogger.Error().Err(event.Err).Msg("Streaming error")
			return
		}
	}

	duration := time.Since(start)

	// Log the response
	sessionLogger.Info().
		Str("model_response", responseBuilder.String()).
		Dur("duration_ms", duration).
		Int("tool_calls", len(toolCallsToExecute)).
		Msg("AI response received")
	logConversation(sessionLogger, openai.ChatMessageRoleAssistant, responseBuilder.String())

	// Execute any tool calls and continue conversation
	if len(toolCallsToExecute) > 0 {
		fmt.Println() // newline before tool execution

		anyHandled := false
		for _, event := range toolCallsToExecute {
			if executeToolCall(session, event.ToolCall, sessionLogger) {
				anyHandled = true
			}
		}

		// Continue conversation with tool results if any tool call was handled
		if anyHandled {
			fmt.Println()
			fmt.Print("⟫ ")
			streamConversation(session, "", false, sessionLogger, canceler)
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

// executeToolCall executes a single tool call and adds result to session.
// Returns true when the tool call is handled.
func executeToolCall(session *chat.Session, toolCall *openai.ToolCall, logger zerolog.Logger) bool {
	toolName := toolCall.Function.Name
	toolArgs := toolCall.Function.Arguments
	toolCallID := toolCall.ID

	if toolName == "read_file" && shouldFillPath(toolCall.Function.Arguments) {
		toolArgs = fillPathFromHistory(session, toolCall, logger)
	}

	trimmedArgs := strings.TrimSpace(toolArgs)
	if trimmedArgs == "" {
		logger.Debug().
			Str("tool_name", toolName).
			Str("tool_call_id", toolCallID).
			Msg("Tool call arguments are empty")
	} else if !json.Valid([]byte(trimmedArgs)) {
		logger.Debug().
			Str("tool_name", toolName).
			Str("tool_call_id", toolCallID).
			Int("tool_args_length", len(toolArgs)).
			Msg("Tool call arguments are not valid JSON")
	}

	logToolCall(logger, toolName, toolArgs, toolCallID)

	// Show what tool is being called
	fmt.Printf("🔧 [%s]", toolName)
	fmt.Println()

	logger.Debug().
		Str("tool_name", toolName).
		Str("tool_call_id", toolCallID).
		Str("tool_args", toolArgs).
		Msg("Executing tool")

	// Execute the tool with approval handling
	result := session.ExecuteToolCallWithApproval(*toolCall)

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
			Str("tool_call_id", toolCallID).
			Err(result.Error).
			Msg("Tool execution failed")
	} else {
		logger.Debug().
			Str("tool_name", toolName).
			Str("tool_call_id", toolCallID).
			Int("result_length", len(result.Result)).
			Msg("Tool executed successfully")
	}
	return true
}

func logToolCall(logger zerolog.Logger, name, args, callID string) {
	logger.Info().
		Str("role", "tool_call").
		Str("tool", name).
		Str("tool_call_id", callID).
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
	candidate := latestPathMention(session)
	if candidate == "" {
		return call.Function.Arguments
	}

	// Use the original text form; validation still happens in the tool.
	call.Function.Arguments = fmt.Sprintf(`{"path": %q}`, candidate)
	logger.Debug().Str("path", candidate).Msg("Filled missing read_file path from conversation")
	return call.Function.Arguments
}

func latestPathMention(session *chat.Session) string {
	history := session.GetHistory()
	for i := len(history) - 1; i >= 0; i-- {
		content := strings.TrimSpace(history[i].Content)
		if content == "" {
			continue
		}
		if candidate := extractPathCandidate(content); candidate != "" {
			return candidate
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
