package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/sashabaranov/go-openai"
)

// Default allow/confirm lists for built-in tools.
var (
	DefaultAllowList   = []string{"get_current_datetime", "read_file", "ls"}
	DefaultConfirmList = []string{"execute_shell_command", "write_file"}
)

// ErrToolNotAllowed indicates a tool is blocked by the current policy.
var ErrToolNotAllowed = errors.New("tool blocked by policy")

// ErrToolRequiresConfirmation indicates a tool requires confirmation before running.
var ErrToolRequiresConfirmation = errors.New("tool requires confirmation")

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

// Permission describes the policy for a tool.
type Permission struct {
	Allowed             bool
	RequireConfirmation bool
}

// Policy configures which tools are allowed and which require confirmation.
type Policy struct {
	Allowed             map[string]bool
	RequireConfirmation map[string]bool
}

// ExecuteOptions controls how tool execution is handled.
type ExecuteOptions struct {
	// Force bypasses policy checks and confirmation requirements (use only after explicit user consent).
	Force bool
}

// Registry holds all available tools with their implementations
type Registry struct {
	mu          sync.RWMutex
	tools       map[string]*Tool
	permissions map[string]Permission
}

// NewRegistry creates a new tool registry and registers all built-in tools
func NewRegistry() *Registry {
	return NewRegistryWithPolicy(DefaultPolicy())
}

// NewRegistryWithPolicy creates a registry with the provided policy.
func NewRegistryWithPolicy(policy Policy) *Registry {
	r := &Registry{
		tools:       make(map[string]*Tool),
		permissions: make(map[string]Permission),
	}

	// Register all built-in tools
	registerBuiltInTools(r)
	r.applyPolicy(DefaultPolicy())
	r.applyPolicy(policy)

	return r
}

// RegisterTool adds a new tool with its implementation to the registry
func (r *Registry) RegisterTool(tool *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
	if _, ok := r.permissions[tool.Name]; !ok {
		// Unknown tools default to blocked + confirmation.
		r.permissions[tool.Name] = Permission{Allowed: false, RequireConfirmation: true}
	}
}

// applyPolicy merges the provided policy into the registry permissions.
func (r *Registry) applyPolicy(policy Policy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name := range r.tools {
		perm, ok := r.permissions[name]
		if !ok {
			perm = Permission{Allowed: false, RequireConfirmation: true}
		}
		if policy.Allowed != nil {
			perm.Allowed = policy.Allowed[name]
		}
		if policy.RequireConfirmation != nil {
			perm.RequireConfirmation = policy.RequireConfirmation[name]
		}
		r.permissions[name] = perm
	}
}

// DefaultPolicy returns the default allow/confirm policy.
func DefaultPolicy() Policy {
	return PolicyFromLists(DefaultAllowList, DefaultConfirmList)
}

// PolicyFromLists builds a policy from allow/confirmation lists.
func PolicyFromLists(allow, confirm []string) Policy {
	allowMap := make(map[string]bool, len(allow))
	for _, name := range allow {
		allowMap[name] = true
	}
	confirmMap := make(map[string]bool, len(confirm))
	for _, name := range confirm {
		confirmMap[name] = true
	}
	return Policy{
		Allowed:             allowMap,
		RequireConfirmation: confirmMap,
	}
}

// GetToolNames returns a list of all tool names
func (r *Registry) GetToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// OpenAITools returns the registry as OpenAI tool definitions.
func (r *Registry) OpenAITools() []openai.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
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
	return r.ExecuteWithOptions(function, args, ExecuteOptions{})
}

// ExecuteWithOptions runs the tool using the provided options.
func (r *Registry) ExecuteWithOptions(function string, args map[string]interface{}, opts ExecuteOptions) *ToolResult {
	result := &ToolResult{
		Function: function,
	}

	tool, exists := r.getTool(function)
	if !exists {
		result.Error = fmt.Errorf("unknown tool: %s", function)
		result.Result = fmt.Sprintf("Error: Tool '%s' not found. Available tools: %v", function, r.GetToolNames())
		return result
	}

	if !opts.Force {
		perm := r.getPermission(function)
		if !perm.Allowed {
			result.Error = fmt.Errorf("%w: %s", ErrToolNotAllowed, function)
			result.Result = fmt.Sprintf("Tool '%s' is blocked by policy. Enable it to proceed.", function)
			return result
		}
		if perm.RequireConfirmation {
			result.Error = fmt.Errorf("%w: %s", ErrToolRequiresConfirmation, function)
			result.Result = fmt.Sprintf("Tool '%s' requires explicit approval before running.", function)
			return result
		}
	}

	result.Result, result.Error = tool.Executor(args)
	return result
}

// ExecuteOpenAIToolCall executes an OpenAI tool call payload.
func (r *Registry) ExecuteOpenAIToolCall(call openai.ToolCall) *ToolResult {
	return r.ExecuteOpenAIToolCallWithOptions(call, ExecuteOptions{})
}

// ExecuteOpenAIToolCallWithOptions executes a tool call with execution options.
func (r *Registry) ExecuteOpenAIToolCallWithOptions(call openai.ToolCall, opts ExecuteOptions) *ToolResult {
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
	return r.ExecuteWithOptions(name, args, opts)
}

// AllowTool marks a tool as allowed and optionally keeps confirmation requirements.
func (r *Registry) AllowTool(name string, requireConfirmation bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	perm := r.permissions[name]
	perm.Allowed = true
	perm.RequireConfirmation = requireConfirmation
	r.permissions[name] = perm
}

// SetAllowed toggles whether a tool is allowed.
func (r *Registry) SetAllowed(name string, allowed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	perm := r.permissions[name]
	perm.Allowed = allowed
	r.permissions[name] = perm
}

// SetRequireConfirmation toggles per-tool confirmation.
func (r *Registry) SetRequireConfirmation(name string, require bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	perm := r.permissions[name]
	perm.RequireConfirmation = require
	r.permissions[name] = perm
}

// GetPermission returns the current permission entry for a tool.
func (r *Registry) GetPermission(name string) Permission {
	return r.getPermission(name)
}

// getTool safely retrieves a tool definition.
func (r *Registry) getTool(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// getPermission safely fetches permissions for a tool.
func (r *Registry) getPermission(name string) Permission {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if perm, ok := r.permissions[name]; ok {
		return perm
	}
	// Default for unknown tools: blocked and requires confirmation.
	return Permission{Allowed: false, RequireConfirmation: true}
}
