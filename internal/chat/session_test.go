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
