package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"promptline/internal/config"
)

func TestStreamEventTypes(t *testing.T) {
	tests := []struct {
		name      string
		eventType StreamEventType
		expected  StreamEventType
	}{
		{"content event", StreamEventContent, StreamEventContent},
		{"tool call event", StreamEventToolCall, StreamEventToolCall},
		{"error event", StreamEventError, StreamEventError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.eventType != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, tt.eventType)
			}
		})
	}
}

func TestStreamEventCreation(t *testing.T) {
	contentEvent := StreamEvent{
		Type:    StreamEventContent,
		Content: "test content",
	}

	if contentEvent.Type != StreamEventContent {
		t.Errorf("Expected StreamEventContent, got %v", contentEvent.Type)
	}

	if contentEvent.Content != "test content" {
		t.Errorf("Expected 'test content', got %s", contentEvent.Content)
	}

	toolCall := &openai.ToolCall{
		ID:   "test",
		Type: openai.ToolTypeFunction,
	}

	toolEvent := StreamEvent{
		Type:     StreamEventToolCall,
		ToolCall: toolCall,
	}

	if toolEvent.Type != StreamEventToolCall {
		t.Errorf("Expected StreamEventToolCall, got %v", toolEvent.Type)
	}

	if toolEvent.ToolCall != toolCall {
		t.Error("ToolCall was not set correctly")
	}
}

func TestAccumulateToolCallWithEmptyID(t *testing.T) {
	toolCalls := make(map[string]*openai.ToolCall)
	argBuilders := make(map[string]*strings.Builder)

	tc := openai.ToolCall{
		ID:   "call_123",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "test_func",
			Arguments: "arg1",
		},
	}

	result := accumulateToolCall(toolCalls, argBuilders, tc)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got %s", result.ID)
	}

	if result.Function.Name != "test_func" {
		t.Errorf("Expected function name 'test_func', got %s", result.Function.Name)
	}
}

func TestAccumulateToolCallMultipleTimes(t *testing.T) {
	toolCalls := make(map[string]*openai.ToolCall)
	argBuilders := make(map[string]*strings.Builder)

	// First call
	tc1 := openai.ToolCall{
		ID:   "call_123",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "test_func",
			Arguments: "part1",
		},
	}

	accumulateToolCall(toolCalls, argBuilders, tc1)

	// Second call with same ID (accumulate arguments)
	tc2 := openai.ToolCall{
		ID: "call_123",
		Function: openai.FunctionCall{
			Arguments: "_part2",
		},
	}

	result := accumulateToolCall(toolCalls, argBuilders, tc2)

	if result.Function.Arguments != "part1_part2" {
		t.Errorf("Expected 'part1_part2', got %s", result.Function.Arguments)
	}
}

func TestFinalizeToolCallsWithValidJSON(t *testing.T) {
	toolCalls := map[string]*openai.ToolCall{
		"call_1": {
			ID:   "call_1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name: "test_func",
			},
		},
	}

	argBuilders := map[string]*strings.Builder{
		"call_1": func() *strings.Builder {
			b := &strings.Builder{}
			b.WriteString(`{"arg": "value"}`)
			return b
		}(),
	}

	results := finalizeToolCalls(toolCalls, argBuilders)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Function.Arguments != `{"arg": "value"}` {
		t.Errorf("Expected valid JSON, got %s", results[0].Function.Arguments)
	}
}

func TestFinalizeToolCallsWithInvalidJSON(t *testing.T) {
	toolCalls := map[string]*openai.ToolCall{
		"call_1": {
			ID:   "call_1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name: "test_func",
			},
		},
	}

	argBuilders := map[string]*strings.Builder{
		"call_1": func() *strings.Builder {
			b := &strings.Builder{}
			b.WriteString(`{invalid json}`)
			return b
		}(),
	}

	results := finalizeToolCalls(toolCalls, argBuilders)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Should default to empty object for invalid JSON
	if results[0].Function.Arguments != "{}" {
		t.Errorf("Expected '{}', got %s", results[0].Function.Arguments)
	}
}

func TestFinalizeToolCallsDropsEmptyNameless(t *testing.T) {
	toolCalls := map[string]*openai.ToolCall{
		"call_1": {
			ID:   "call_1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name: "", // Empty name
			},
		},
		"call_2": {
			ID:   "call_2",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name: "valid_func",
			},
		},
	}

	argBuilders := map[string]*strings.Builder{
		"call_1": &strings.Builder{}, // Empty args
		"call_2": func() *strings.Builder {
			b := &strings.Builder{}
			b.WriteString("{}")
			return b
		}(),
	}

	results := finalizeToolCalls(toolCalls, argBuilders)

	// Should only have call_2 (call_1 dropped because no name and no args)
	if len(results) != 1 {
		t.Fatalf("Expected 1 result (dropped empty call), got %d", len(results))
	}

	if results[0].ID != "call_2" {
		t.Errorf("Expected call_2 to remain, got %s", results[0].ID)
	}
}

func TestFinalizeToolCallsDefaultsUnknownName(t *testing.T) {
	toolCalls := map[string]*openai.ToolCall{
		"call_1": {
			ID:   "call_1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name: "", // Empty name
			},
		},
	}

	argBuilders := map[string]*strings.Builder{
		"call_1": func() *strings.Builder {
			b := &strings.Builder{}
			b.WriteString(`{"has": "args"}`)
			return b
		}(),
	}

	results := finalizeToolCalls(toolCalls, argBuilders)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	// Should default to unknown_tool when name is empty but args exist
	if results[0].Function.Name != "unknown_tool" {
		t.Errorf("Expected 'unknown_tool', got %s", results[0].Function.Name)
	}
}

func TestMessagesSnapshotConcurrency(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := NewSession(cfg)

	// Add messages concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			session.AddMessage(openai.ChatMessageRoleUser, "Concurrent message")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have all messages
	snapshot := session.MessagesSnapshot()
	if len(snapshot) < 11 { // 1 system + 10 user
		t.Errorf("Expected at least 11 messages, got %d", len(snapshot))
	}
}

func TestStreamResponseWithContextCancellation(t *testing.T) {
	cfg := &config.Config{
		APIKey: "test-key",
		Model:  "gpt-4o-mini",
	}

	session := NewSession(cfg)
	events := make(chan StreamEvent, 10)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This should handle cancellation gracefully
	go session.StreamResponseWithContext(ctx, "test", true, events)

	// Wait a bit for the goroutine to process
	time.Sleep(100 * time.Millisecond)

	// Channel should eventually close
	select {
	case event, ok := <-events:
		if ok && event.Type == StreamEventError {
			// Expected - context cancelled error
			if !strings.Contains(event.Err.Error(), "context canceled") {
				t.Errorf("Expected context canceled error, got: %v", event.Err)
			}
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for event")
	}
}
