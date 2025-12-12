package chat

import (
	"testing"

	"github.com/sashabaranov/go-openai"
	"promptline/internal/config"
	"promptline/internal/tools"
)

// TestAddToolResultMessage verifies tool result messages are added correctly
func TestAddToolResultMessage(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "test-model",
	}
	session := NewSession(cfg)

	toolCall := openai.ToolCall{
		ID:   "call-123",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "test_tool",
			Arguments: `{"arg": "value"}`,
		},
	}

	result := &tools.ToolResult{
		Function: "test_tool",
		Result:   "tool result content",
		Error:    nil,
	}

	// Initial state: should have 1 system message
	if len(session.Messages) != 1 {
		t.Fatalf("expected 1 system message, got %d", len(session.Messages))
	}

	session.AddToolResultMessage(toolCall, result)

	// Should now have 2 messages
	if len(session.Messages) != 2 {
		t.Fatalf("expected 2 messages after adding tool result, got %d", len(session.Messages))
	}

	toolMsg := session.Messages[1]
	if toolMsg.Role != openai.ChatMessageRoleTool {
		t.Errorf("expected role 'tool', got %s", toolMsg.Role)
	}
	if toolMsg.ToolCallID != "call-123" {
		t.Errorf("expected tool_call_id 'call-123', got %s", toolMsg.ToolCallID)
	}
	if toolMsg.Name != "test_tool" {
		t.Errorf("expected name 'test_tool', got %s", toolMsg.Name)
	}
	if toolMsg.Content == "" {
		t.Error("expected non-empty content")
	}
}

// TestAddToolResultMessageWithError verifies error handling
func TestAddToolResultMessageWithError(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "test-model",
	}
	session := NewSession(cfg)

	toolCall := openai.ToolCall{
		ID:   "call-456",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "failing_tool",
			Arguments: `{}`,
		},
	}

	result := &tools.ToolResult{
		Function: "failing_tool",
		Error:    tools.ErrToolNotAllowed,
	}

	session.AddToolResultMessage(toolCall, result)

	toolMsg := session.Messages[1]
	if toolMsg.Role != openai.ChatMessageRoleTool {
		t.Errorf("expected role 'tool', got %s", toolMsg.Role)
	}
	// Content should include error information
	if toolMsg.Content == "" {
		t.Error("expected non-empty content for error result")
	}
}

// TestToolCallMessageSequence verifies the correct message sequence
func TestToolCallMessageSequence(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "test-model",
	}
	session := NewSession(cfg)

	// Add user message
	session.AddMessage(openai.ChatMessageRoleUser, "What time is it?")

	// Add assistant message with tool call
	toolCalls := []openai.ToolCall{
		{
			ID:   "call-789",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "get_current_datetime",
				Arguments: `{}`,
			},
		},
	}
	session.AddAssistantMessage("", toolCalls)

	// Add tool result
	result := &tools.ToolResult{
		Function: "get_current_datetime",
		Result:   "2025-12-12T00:00:00Z",
	}
	session.AddToolResultMessage(toolCalls[0], result)

	// Verify message sequence
	if len(session.Messages) != 4 {
		t.Fatalf("expected 4 messages (system, user, assistant, tool), got %d", len(session.Messages))
	}

	// Check sequence
	if session.Messages[0].Role != openai.ChatMessageRoleSystem {
		t.Errorf("message 0: expected system, got %s", session.Messages[0].Role)
	}
	if session.Messages[1].Role != openai.ChatMessageRoleUser {
		t.Errorf("message 1: expected user, got %s", session.Messages[1].Role)
	}
	if session.Messages[2].Role != openai.ChatMessageRoleAssistant {
		t.Errorf("message 2: expected assistant, got %s", session.Messages[2].Role)
	}
	if session.Messages[3].Role != openai.ChatMessageRoleTool {
		t.Errorf("message 3: expected tool, got %s", session.Messages[3].Role)
	}

	// Verify assistant has tool calls
	if len(session.Messages[2].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call in assistant message, got %d", len(session.Messages[2].ToolCalls))
	}

	// Verify tool message references the call
	if session.Messages[3].ToolCallID != "call-789" {
		t.Errorf("tool message should reference call-789, got %s", session.Messages[3].ToolCallID)
	}
}

// TestMultipleToolCalls verifies handling of multiple tool calls
func TestMultipleToolCalls(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "test-model",
	}
	session := NewSession(cfg)

	// Add user message
	session.AddMessage(openai.ChatMessageRoleUser, "Show me files and current time")

	// Add assistant message with multiple tool calls
	toolCalls := []openai.ToolCall{
		{
			ID:   "call-1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "ls",
				Arguments: `{"path": "."}`,
			},
		},
		{
			ID:   "call-2",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "get_current_datetime",
				Arguments: `{}`,
			},
		},
	}
	session.AddAssistantMessage("", toolCalls)

	// Add results for both tool calls
	result1 := &tools.ToolResult{
		Function: "ls",
		Result:   "file1.txt\nfile2.txt",
	}
	session.AddToolResultMessage(toolCalls[0], result1)

	result2 := &tools.ToolResult{
		Function: "get_current_datetime",
		Result:   "2025-12-12T00:00:00Z",
	}
	session.AddToolResultMessage(toolCalls[1], result2)

	// Should have: system, user, assistant, tool1, tool2
	if len(session.Messages) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(session.Messages))
	}

	// Verify both tool messages
	if session.Messages[3].ToolCallID != "call-1" {
		t.Errorf("expected tool message 3 to reference call-1, got %s", session.Messages[3].ToolCallID)
	}
	if session.Messages[4].ToolCallID != "call-2" {
		t.Errorf("expected tool message 4 to reference call-2, got %s", session.Messages[4].ToolCallID)
	}
}

// TestToolResultsInHistory verifies tool results appear in history
func TestToolResultsInHistory(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "test-model",
	}
	session := NewSession(cfg)

	// Add a complete conversation with tool call
	session.AddMessage(openai.ChatMessageRoleUser, "test")
	session.AddAssistantMessage("", []openai.ToolCall{
		{
			ID:   "call-x",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "test_tool",
				Arguments: `{}`,
			},
		},
	})
	session.AddToolResultMessage(openai.ToolCall{
		ID:   "call-x",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "test_tool",
			Arguments: `{}`,
		},
	}, &tools.ToolResult{
		Function: "test_tool",
		Result:   "result",
	})

	history := session.GetHistory()
	
	// Should have user, assistant, and tool messages
	if len(history) != 3 {
		t.Fatalf("expected 3 messages in history, got %d", len(history))
	}

	// Verify tool result is in history
	foundToolMessage := false
	for _, msg := range history {
		if msg.Role == openai.ChatMessageRoleTool {
			foundToolMessage = true
			break
		}
	}
	if !foundToolMessage {
		t.Error("tool result message not found in history")
	}
}
