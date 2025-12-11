package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
	"promptline/internal/tools"
)

func TestAddToolResultMessageStoresTOON(t *testing.T) {
	s := &Session{
		ToolRegistry: tools.NewRegistry(),
	}

	call := openai.ToolCall{
		ID:   "call-1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: `{"path": "."}`,
		},
	}
	result := &tools.ToolResult{
		Function: "ls",
		Result:   "ok",
	}

	s.AddToolResultMessage(call, result)

	if len(s.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.Messages))
	}

	msg := s.Messages[0]
	if msg.Role != openai.ChatMessageRoleTool {
		t.Fatalf("expected role tool, got %s", msg.Role)
	}
	if msg.ToolCallID != call.ID {
		t.Fatalf("expected tool_call_id %s, got %s", call.ID, msg.ToolCallID)
	}

	if !strings.Contains(msg.Content, "result") || !strings.Contains(msg.Content, "ok") {
		t.Fatalf("expected TOON content to include result and value, got %q", msg.Content)
	}
}

func TestAccumulateToolCall(t *testing.T) {
	toolCalls := map[string]*openai.ToolCall{}
	argBuilders := map[string]*strings.Builder{}

	// first chunk with name
	entry := accumulateToolCall(toolCalls, argBuilders, openai.ToolCall{
		ID:   "1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: `{"path":`,
		},
	})
	toolCalls["1"] = entry

	// second chunk with arguments continued
	entry = accumulateToolCall(toolCalls, argBuilders, openai.ToolCall{
		ID:   "1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Arguments: `"."}`,
		},
	})
	toolCalls["1"] = entry

	call := toolCalls["1"]
	if call == nil {
		t.Fatal("expected tool call stored")
	}
	if call.Function.Name != "ls" {
		t.Fatalf("expected function name ls, got %s", call.Function.Name)
	}
	if call.Function.Arguments != `{"path":"."}` {
		t.Fatalf("expected merged arguments JSON, got %s", call.Function.Arguments)
	}
}

func TestAccumulateToolCallMissingNameDefaults(t *testing.T) {
	toolCalls := map[string]*openai.ToolCall{}
	argBuilders := map[string]*strings.Builder{}

	entry := accumulateToolCall(toolCalls, argBuilders, openai.ToolCall{
		ID:   "1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "",
			Arguments: `{"x":1}`,
		},
	})
	toolCalls["1"] = entry

	call := toolCalls["1"]
	if call == nil {
		t.Fatal("expected tool call stored")
	}
	if call.Function.Name != "" {
		t.Fatalf("expected empty name to remain until finalization, got %s", call.Function.Name)
	}
	if call.Function.Arguments != `{"x":1}` {
		t.Fatalf("expected arguments copied, got %s", call.Function.Arguments)
	}
}

func TestFinalizeToolCallsEnsuresNameAndJSONArgs(t *testing.T) {
	toolCalls := map[string]*openai.ToolCall{}
	argBuilders := map[string]*strings.Builder{}

	// Empty name and args should be discarded.
	entry := accumulateToolCall(toolCalls, argBuilders, openai.ToolCall{
		ID:   "1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "",
			Arguments: "",
		},
	})
	toolCalls["1"] = entry

	// Another call with args but missing name should be kept and normalized.
	entry2 := accumulateToolCall(toolCalls, argBuilders, openai.ToolCall{
		ID:   "2",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "",
			Arguments: `{"x":1}`,
		},
	})
	toolCalls["2"] = entry2

	final := finalizeToolCalls(toolCalls, argBuilders)
	if len(final) != 1 {
		t.Fatalf("expected 1 call kept, got %d", len(final))
	}
	call := final[0]
	if call.Function.Name != "unknown_tool" {
		t.Fatalf("expected unknown_tool fallback, got %s", call.Function.Name)
	}
	if call.Function.Arguments != `{"x":1}` {
		t.Fatalf("expected args to preserve JSON, got %q", call.Function.Arguments)
	}

	// When name exists but args are empty, default to {} for JSON validity.
	entry3 := accumulateToolCall(toolCalls, argBuilders, openai.ToolCall{
		ID:   "3",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: "",
		},
	})
	toolCalls["3"] = entry3
	final = finalizeToolCalls(toolCalls, argBuilders)
	foundEmpty := false
	for _, c := range final {
		if c.Function.Name == "ls" && c.Function.Arguments == "{}" {
			foundEmpty = true
		}
	}
	if !foundEmpty {
		t.Fatalf("expected ls call with empty args coerced to {}")
	}
}

func TestAddToolResultMessageIncludesError(t *testing.T) {
	s := &Session{
		ToolRegistry: tools.NewRegistry(),
	}

	call := openai.ToolCall{
		ID:   "call-err",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "",
			Arguments: `{"path": "."}`,
		},
	}
	result := &tools.ToolResult{
		Function: "ls",
		Error:    fmt.Errorf("boom"),
	}

	s.AddToolResultMessage(call, result)

	if len(s.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.Messages))
	}
	msg := s.Messages[0]
	if msg.Role != openai.ChatMessageRoleTool {
		t.Fatalf("expected tool role, got %s", msg.Role)
	}
	if msg.Name == "" {
		t.Fatalf("expected fallback name to be set")
	}
	if !strings.Contains(msg.Content, "error") || !strings.Contains(msg.Content, "boom") {
		t.Fatalf("expected TOON content with error, got %q", msg.Content)
	}
}

func TestGetHistoryEmpty(t *testing.T) {
	s := &Session{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "system prompt"},
		},
	}

	history := s.GetHistory()
	if len(history) != 0 {
		t.Fatalf("expected empty history (system message excluded), got %d messages", len(history))
	}
}

func TestGetHistoryWithMessages(t *testing.T) {
	s := &Session{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "system prompt"},
			{Role: openai.ChatMessageRoleUser, Content: "hello"},
			{Role: openai.ChatMessageRoleAssistant, Content: "hi there"},
		},
	}

	history := s.GetHistory()
	if len(history) != 2 {
		t.Fatalf("expected 2 messages in history, got %d", len(history))
	}
	if history[0].Role != openai.ChatMessageRoleUser {
		t.Errorf("expected first message to be user, got %s", history[0].Role)
	}
	if history[1].Role != openai.ChatMessageRoleAssistant {
		t.Errorf("expected second message to be assistant, got %s", history[1].Role)
	}
}

func TestClearHistoryKeepsSystemMessage(t *testing.T) {
	s := &Session{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "system prompt"},
			{Role: openai.ChatMessageRoleUser, Content: "hello"},
			{Role: openai.ChatMessageRoleAssistant, Content: "hi there"},
		},
	}

	s.ClearHistory()

	if len(s.Messages) != 1 {
		t.Fatalf("expected 1 message after clear (system), got %d", len(s.Messages))
	}
	if s.Messages[0].Role != openai.ChatMessageRoleSystem {
		t.Errorf("expected system message to remain, got %s", s.Messages[0].Role)
	}
}

func TestAddMessage(t *testing.T) {
	s := &Session{
		Messages: []openai.ChatCompletionMessage{},
	}

	s.AddMessage(openai.ChatMessageRoleUser, "test message")

	if len(s.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.Messages))
	}
	if s.Messages[0].Role != openai.ChatMessageRoleUser {
		t.Errorf("expected user role, got %s", s.Messages[0].Role)
	}
	if s.Messages[0].Content != "test message" {
		t.Errorf("expected content 'test message', got %s", s.Messages[0].Content)
	}
}

func TestAddAssistantMessage(t *testing.T) {
	s := &Session{
		Messages: []openai.ChatCompletionMessage{},
	}

	toolCalls := []openai.ToolCall{
		{
			ID:   "call-1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "ls",
				Arguments: `{"path": "."}`,
			},
		},
	}

	s.AddAssistantMessage("response text", toolCalls)

	if len(s.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.Messages))
	}
	msg := s.Messages[0]
	if msg.Role != openai.ChatMessageRoleAssistant {
		t.Errorf("expected assistant role, got %s", msg.Role)
	}
	if msg.Content != "response text" {
		t.Errorf("expected content 'response text', got %s", msg.Content)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "ls" {
		t.Errorf("expected tool call name 'ls', got %s", msg.ToolCalls[0].Function.Name)
	}
}

func TestMessagesSnapshotIsCopy(t *testing.T) {
	s := &Session{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "system"},
			{Role: openai.ChatMessageRoleUser, Content: "user"},
		},
	}

	snapshot := s.MessagesSnapshot()

	// Modify snapshot
	snapshot[1].Content = "modified"

	// Original should be unchanged
	if s.Messages[1].Content == "modified" {
		t.Error("MessagesSnapshot should return a copy, not reference to original")
	}
	if s.Messages[1].Content != "user" {
		t.Errorf("expected original content 'user', got %s", s.Messages[1].Content)
	}
}
