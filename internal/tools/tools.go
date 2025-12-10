package tools

import (
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// ExecutorFunc is the function signature for tool implementations
type ExecutorFunc func(args map[string]interface{}) (string, error)

// Tool represents a callable tool/function with its implementation
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Executor    ExecutorFunc           `json:"-"` // Function to execute the tool
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Function string
	Result   string
	Error    error
}

// Registry holds all available tools with their implementations
type Registry struct {
	tools map[string]*Tool
}

// NewRegistry creates a new tool registry and registers all built-in tools
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]*Tool),
	}

	// Register all built-in tools
	registerBuiltInTools(r)

	return r
}

// RegisterTool adds a new tool with its implementation to the registry
func (r *Registry) RegisterTool(tool *Tool) {
	r.tools[tool.Name] = tool
}

// GetToolNames returns a list of all tool names
func (r *Registry) GetToolNames() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// OpenAITools returns the registry as OpenAI tool definitions.
func (r *Registry) OpenAITools() []openai.Tool {
	defs := make([]openai.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	return defs
}

// Execute runs the specified tool with given arguments.
func (r *Registry) Execute(function string, args map[string]interface{}) *ToolResult {
	result := &ToolResult{
		Function: function,
	}

	tool, exists := r.tools[function]
	if !exists {
		result.Error = fmt.Errorf("unknown tool: %s", function)
		result.Result = fmt.Sprintf("Error: Tool '%s' not found. Available tools: %v", function, r.GetToolNames())
		return result
	}

	result.Result, result.Error = tool.Executor(args)
	return result
}

// ExecuteOpenAIToolCall executes an OpenAI tool call payload.
func (r *Registry) ExecuteOpenAIToolCall(call openai.ToolCall) *ToolResult {
	args := map[string]interface{}{}
	if call.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return &ToolResult{
				Function: call.Function.Name,
				Error:    fmt.Errorf("invalid tool arguments: %w", err),
				Result:   "",
			}
		}
	}
	name := call.Function.Name
	if name == "" {
		name = "unknown_tool"
		return &ToolResult{
			Function: name,
			Error:    fmt.Errorf("tool call missing function name"),
			Result:   "",
		}
	}
	return r.Execute(name, args)
}
