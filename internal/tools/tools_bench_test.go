package tools

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

// BenchmarkToolRegistration measures tool registration performance
func BenchmarkToolRegistration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry := NewRegistry()
		_ = registry
	}
}

// BenchmarkGetPermission measures permission check performance
func BenchmarkGetPermission(b *testing.B) {
	registry := NewRegistry()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.getPermission("ls")
	}
}

// BenchmarkExecuteOpenAIToolCall measures tool execution overhead
func BenchmarkExecuteOpenAIToolCall(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("ls", true)
	
	toolCall := openai.ToolCall{
		ID:   "test-call",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: `{"path": "."}`,
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.ExecuteOpenAIToolCall(toolCall)
	}
}

// BenchmarkExecuteMultipleTools measures batch tool execution
func BenchmarkExecuteMultipleTools(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("ls", true)
	registry.SetAllowed("get_current_datetime", true)
	
	toolCalls := []openai.ToolCall{
		{
			ID:   "call1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "ls",
				Arguments: `{"path": "."}`,
			},
		},
		{
			ID:   "call2",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "get_current_datetime",
				Arguments: `{}`,
			},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range toolCalls {
			_ = registry.ExecuteOpenAIToolCall(tc)
		}
	}
}

// BenchmarkFormatToolResult measures result formatting performance
func BenchmarkFormatToolResult(b *testing.B) {
	toolCall := openai.ToolCall{
		ID:   "test-call",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "ls",
			Arguments: `{"path": "."}`,
		},
	}
	
	result := &ToolResult{
		Function: "ls",
		Result:   "file1.txt\nfile2.txt\nfile3.txt",
		Error:    nil,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatToolResult(toolCall, result, false)
	}
}

// BenchmarkConcurrentToolExecution measures concurrent tool execution
func BenchmarkConcurrentToolExecution(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("get_current_datetime", true)
	
	toolCall := openai.ToolCall{
		ID:   "test-call",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "get_current_datetime",
			Arguments: `{}`,
		},
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = registry.ExecuteOpenAIToolCall(toolCall)
		}
	})
}

// BenchmarkPolicyApplicationBench measures policy application overhead
func BenchmarkPolicyApplicationBench(b *testing.B) {
	policy := Policy{
		Allowed: map[string]bool{
			"tool1": true,
			"tool2": true,
			"tool3": false,
		},
		RequireConfirmation: map[string]bool{
			"tool2": true,
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry := NewRegistry()
		registry.applyPolicy(policy)
	}
}

// BenchmarkOpenAIToolsConversion measures OpenAI tools conversion
func BenchmarkOpenAIToolsConversion(b *testing.B) {
	registry := NewRegistry()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.OpenAITools()
	}
}
