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

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Default permissions are ask unless explicitly set by policy.

// ExecutorFunc is the function signature for tool implementations.
type ExecutorFunc func(ctx context.Context, args map[string]interface{}) (string, error)

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Function string
	Result   string
	Error    error
}

// Permission describes the policy for a tool.
type PermissionLevel string

const (
	PermissionAllow PermissionLevel = "allow"
	PermissionAsk   PermissionLevel = "ask"
	PermissionDeny  PermissionLevel = "deny"
)

// Permission describes the policy for a tool.
type Permission struct {
	Level PermissionLevel
}

// Policy configures which tools are allowed, asked, or denied.
type Policy struct {
	Allow map[string]bool
	Ask   map[string]bool
	Deny  map[string]bool
}

// ExecuteOptions controls how tool execution is handled.
type ExecuteOptions struct {
	// Force bypasses policy checks and confirmation requirements (use only after explicit user consent).
	Force bool
	// DryRun validates tool arguments but skips execution.
	DryRun bool
}

// Registry holds all available tools with their implementations.
//
// Thread-safety: Registry is safe for concurrent use from multiple goroutines.
// All access to tools and permissions maps is protected by a RWMutex.
// Read operations (getTool, getPermission, GetToolNames, GetTools, OpenAITools)
// use RLock for concurrent reads. Write operations (RegisterTool, applyPolicy,
// AllowTool, SetAllowed, SetRequireConfirmation) use Lock for exclusive access.
type Registry struct {
	mu           sync.RWMutex
	tools        map[string]Tool
	permissions  map[string]Permission
	rateLimits   RateLimitConfig
	rateLimiters map[string]*toolRateLimiter
	timeouts     TimeoutConfig
}

// NewRegistry creates a new tool registry and registers all built-in tools
func NewRegistry() *Registry {
	return NewRegistryWithPolicy(DefaultPolicy())
}

// NewRegistryWithPolicy creates a registry with the provided policy.
func NewRegistryWithPolicy(policy Policy) *Registry {
	r := &Registry{
		tools:        make(map[string]Tool),
		permissions:  make(map[string]Permission),
		rateLimits:   DefaultRateLimitConfig(),
		rateLimiters: make(map[string]*toolRateLimiter),
		timeouts:     DefaultTimeoutConfig(),
	}

	// Register all built-in tools
	registerBuiltInTools(r)
	r.applyPolicy(DefaultPolicy())
	r.applyPolicy(policy)

	return r
}

// RegisterTool adds a new tool with its implementation to the registry
func (r *Registry) RegisterTool(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool is nil")
	}
	name := tool.Name()
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("tool name is required")
	}
	if strings.TrimSpace(tool.Version()) == "" {
		return fmt.Errorf("tool %q must declare a version", name)
	}
	if !tool.CompatibleWith(HostAPIVersion) {
		return fmt.Errorf("tool %q is incompatible with host API %s", name, HostAPIVersion)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = tool
	if _, ok := r.permissions[name]; !ok {
		// Default to ask unless configured otherwise.
		r.permissions[name] = Permission{Level: PermissionAsk}
	}
	return nil
}

// RegisterPlugin registers all tools from a plugin.
func (r *Registry) RegisterPlugin(plugin ToolPlugin) error {
	if plugin == nil {
		return fmt.Errorf("plugin is nil")
	}
	if strings.TrimSpace(plugin.Name()) == "" {
		return fmt.Errorf("plugin name is required")
	}
	if strings.TrimSpace(plugin.Version()) == "" {
		return fmt.Errorf("plugin %q must declare a version", plugin.Name())
	}

	for _, tool := range plugin.Tools() {
		if err := r.RegisterTool(tool); err != nil {
			return fmt.Errorf("plugin %q failed to register tool: %w", plugin.Name(), err)
		}
	}
	return nil
}

// applyPolicy merges the provided policy into the registry permissions.
func (r *Registry) applyPolicy(policy Policy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name := range r.tools {
		perm, ok := r.permissions[name]
		if !ok {
			perm = Permission{Level: PermissionAsk}
		}
		perm.Level = applyPolicyLevel(perm.Level, name, policy)
		r.permissions[name] = perm
	}
}

// DefaultPolicy returns the default allow/ask/deny policy.
func DefaultPolicy() Policy {
	return PolicyFromLists(nil, nil, nil)
}

// PolicyFromLists builds a policy from allow/ask/deny lists.
func PolicyFromLists(allow, ask, deny []string) Policy {
	allowMap := make(map[string]bool, len(allow))
	for _, name := range allow {
		allowMap[name] = true
	}
	askMap := make(map[string]bool, len(ask))
	for _, name := range ask {
		askMap[name] = true
	}
	denyMap := make(map[string]bool, len(deny))
	for _, name := range deny {
		denyMap[name] = true
	}
	return Policy{
		Allow: allowMap,
		Ask:   askMap,
		Deny:  denyMap,
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

// GetTools returns a copy of all registered tools (thread-safe)
func (r *Registry) GetTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		list = append(list, tool)
	}
	return list
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
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
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
		switch perm.Level {
		case PermissionAllow:
			// Allowed with no prompt.
		case PermissionAsk:
			result.Error = fmt.Errorf("%w: %s", ErrToolRequiresConfirmation, function)
			result.Result = fmt.Sprintf("Tool '%s' requires user approval before running.", function)
			return result
		case PermissionDeny:
			result.Error = fmt.Errorf("%w: %s", ErrToolNotAllowed, function)
			result.Result = fmt.Sprintf("Tool '%s' is denied by policy. Enable it to proceed.", function)
			return result
		default:
			result.Error = fmt.Errorf("%w: %s", ErrToolRequiresConfirmation, function)
			result.Result = fmt.Sprintf("Tool '%s' requires user approval before running.", function)
			return result
		}
	}

	if err := r.checkRateLimit(function); err != nil {
		result.Error = err
		result.Result = fmt.Sprintf("Error: %v", err)
		return result
	}

	ctx := context.Background()
	timeout := r.getTimeout(function)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if opts.DryRun {
		if err := tool.Validate(args); err != nil {
			result.Error = fmt.Errorf("%w: %v", ErrInvalidArguments, err)
			result.Result = fmt.Sprintf("Error: %v", result.Error)
			return result
		}
		result.Result = formatDryRunResult(function, args)
		return result
	}

	result.Result, result.Error = tool.Execute(ctx, args)
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
	perm.Level = PermissionAllow
	if requireConfirmation {
		perm.Level = PermissionAsk
	}
	r.permissions[name] = perm
}

// SetAllowed toggles whether a tool is allowed.
func (r *Registry) SetAllowed(name string, allowed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	perm := r.permissions[name]
	if allowed {
		perm.Level = PermissionAllow
	} else {
		perm.Level = PermissionDeny
	}
	r.permissions[name] = perm
}

// SetRequireConfirmation toggles per-tool confirmation.
func (r *Registry) SetRequireConfirmation(name string, require bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	perm := r.permissions[name]
	if require {
		perm.Level = PermissionAsk
	} else {
		perm.Level = PermissionAllow
	}
	r.permissions[name] = perm
}

// GetPermission returns the current permission entry for a tool.
func (r *Registry) GetPermission(name string) Permission {
	return r.getPermission(name)
}

// ConfigureRateLimits updates rate limiting configuration for the registry.
func (r *Registry) ConfigureRateLimits(config RateLimitConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, limiter := range r.rateLimiters {
		if limiter != nil {
			limiter.Stop()
		}
	}
	r.rateLimits = config
	r.rateLimiters = make(map[string]*toolRateLimiter)
}

// ConfigureTimeouts updates tool execution timeouts.
func (r *Registry) ConfigureTimeouts(config TimeoutConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.timeouts = config
}

// getTool safely retrieves a tool definition.
func (r *Registry) getTool(name string) (Tool, bool) {
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
	// Default for unknown tools: ask.
	return Permission{Level: PermissionAsk}
}

func (r *Registry) checkRateLimit(name string) error {
	limiter := r.getRateLimiter(name)
	if limiter == nil {
		return nil
	}
	return limiter.Allow()
}

func (r *Registry) getRateLimiter(name string) *toolRateLimiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limiter, ok := r.rateLimiters[name]; ok {
		return limiter
	}

	rate := r.rateLimits.DefaultPerMinute
	if r.rateLimits.PerTool != nil {
		if perTool, ok := r.rateLimits.PerTool[name]; ok {
			rate = perTool
		}
	}

	var cooldown time.Duration
	if r.rateLimits.Cooldowns != nil {
		if perTool, ok := r.rateLimits.Cooldowns[name]; ok {
			cooldown = perTool
		}
	}

	limiter := newToolRateLimiter(rate, cooldown)
	r.rateLimiters[name] = limiter
	return limiter
}

func (r *Registry) getTimeout(name string) time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.timeouts.TimeoutForTool(name)
}

func applyPolicyLevel(current PermissionLevel, name string, policy Policy) PermissionLevel {
	level := current
	if policy.Deny != nil && policy.Deny[name] {
		level = PermissionDeny
	} else if policy.Ask != nil && policy.Ask[name] {
		level = PermissionAsk
	} else if policy.Allow != nil && policy.Allow[name] {
		level = PermissionAllow
	}
	return level
}

// FormatToolResult creates a user-friendly display of tool execution
func FormatToolResult(toolCall openai.ToolCall, result *ToolResult, truncate bool) string {
	var argsStr string
	if toolCall.Function.Arguments != "" {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil && len(args) > 0 {
			parts := make([]string, 0, len(args))
			for key, value := range args {
				parts = append(parts, fmt.Sprintf("%s=%v", key, value))
			}
			argsStr = strings.Join(parts, ", ")
		} else {
			argsStr = toolCall.Function.Arguments
		}
	}

	sb := getBuilder()
	defer putBuilder(sb)
	if argsStr != "" {
		sb.WriteString(fmt.Sprintf("🔧 Executed: %s(%s)\n", toolCall.Function.Name, argsStr))
	} else {
		sb.WriteString(fmt.Sprintf("🔧 Executed: %s()\n", toolCall.Function.Name))
	}

	if result.Error != nil {
		sb.WriteString(fmt.Sprintf("❌ Error: %v\n", result.Error))
	} else {
		displayResult, truncated := sanitizeToolOutput(result.Result)
		if truncate {
			var shortTruncated bool
			displayResult, shortTruncated = truncateString(displayResult, 200)
			truncated = truncated || shortTruncated
		}
		if truncated {
			displayResult += "..."
		}
		sb.WriteString(fmt.Sprintf("✓ Result:\n%s\n", displayResult))
	}
	return sb.String()
}

func formatDryRunResult(function string, args map[string]interface{}) string {
	if len(args) == 0 {
		return fmt.Sprintf("Dry run: %s()", function)
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("Dry run: %s(%v)", function, args)
	}
	return fmt.Sprintf("Dry run: %s(%s)", function, string(encoded))
}
