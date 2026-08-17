package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAITools is a temporary provider adapter. The toolbox core is provider-neutral.
func (r *Registry) OpenAITools() []openai.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]openai.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: tool.Name(), Description: tool.Description(), Parameters: tool.Parameters()}})
	}
	return defs
}

func (r *Registry) ExecuteOpenAIToolCall(call openai.ToolCall) *ToolResult {
	return r.ExecuteOpenAIToolCallWithOptions(call, ExecuteOptions{})
}

func (r *Registry) ExecuteOpenAIToolCallWithOptions(call openai.ToolCall, opts ExecuteOptions) *ToolResult {
	return r.ExecuteOpenAIToolCallContextWithOptions(context.TODO(), call, opts)
}

// ExecuteOpenAIToolCallContextWithOptions is the context-aware OpenAI adapter.
func (r *Registry) ExecuteOpenAIToolCallContextWithOptions(ctx context.Context, call openai.ToolCall, opts ExecuteOptions) *ToolResult {
	args := map[string]interface{}{}
	if call.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return &ToolResult{Function: call.Function.Name, Error: fmt.Errorf("invalid tool arguments: %w", err)}
		}
	}
	if call.Function.Name == "" {
		return &ToolResult{Function: "unknown_tool", Error: fmt.Errorf("tool call missing function name")}
	}
	return r.ExecuteContextWithOptions(ctx, call.Function.Name, args, opts)
}

func FormatToolResult(call openai.ToolCall, result *ToolResult, truncate bool) string {
	return FormatToolCallResult(call.Function.Name, call.Function.Arguments, result, truncate)
}
