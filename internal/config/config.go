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

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"promptline/internal/tools"
)

// Config represents the application configuration
type Config struct {
	APIKey             string            `json:"api_key"`
	APIURL             string            `json:"api_url,omitempty"`
	Model              string            `json:"model"`
	Temperature        *float32          `json:"temperature,omitempty"`
	MaxTokens          *int              `json:"max_tokens,omitempty"`
	Tools              ToolSettings      `json:"tools,omitempty"`
	ToolLimits         ToolLimits        `json:"tool_limits,omitempty"`
	ToolPathWhitelist  []string          `json:"tool_path_whitelist,omitempty"`
	ToolRateLimits     ToolRateLimits    `json:"tool_rate_limits,omitempty"`
	ToolTimeouts       ToolTimeouts      `json:"tool_timeouts,omitempty"`
	ToolOutputFilters  ToolOutputFilters `json:"tool_output_filters,omitempty"`
	HistoryFile        string            `json:"history_file,omitempty"`
	CommandHistoryFile string            `json:"command_history_file,omitempty"`
	HistoryMaxMessages int               `json:"history_max_messages,omitempty"`
}

// ToolSettings describes tool allow/ask/deny lists.
type ToolSettings struct {
	Allow               []string `json:"allow"`
	Ask                 []string `json:"ask,omitempty"`
	Deny                []string `json:"deny,omitempty"`
	RequireConfirmation []string `json:"require_confirmation,omitempty"`
}

// ToolLimits configures resource limits for tool execution.
type ToolLimits struct {
	MaxFileSizeBytes    int64 `json:"max_file_size_bytes,omitempty"`
	MaxDirectoryDepth   int   `json:"max_directory_depth,omitempty"`
	MaxDirectoryEntries int   `json:"max_directory_entries,omitempty"`
}

// ToolRateLimits configures tool rate limits and cooldowns.
type ToolRateLimits struct {
	DefaultPerMinute int            `json:"default_per_minute,omitempty"`
	PerTool          map[string]int `json:"per_tool,omitempty"`
	CooldownSeconds  map[string]int `json:"cooldown_seconds,omitempty"`
}

// ToolTimeouts configures tool execution timeouts.
type ToolTimeouts struct {
	DefaultSeconds int            `json:"default_seconds,omitempty"`
	PerToolSeconds map[string]int `json:"per_tool_seconds,omitempty"`
}

// ToolOutputFilters configures output sanitization for tool results.
type ToolOutputFilters struct {
	MaxChars     int  `json:"max_chars,omitempty"`
	StripANSI    bool `json:"strip_ansi,omitempty"`
	StripControl bool `json:"strip_control,omitempty"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	defaultModel := "gpt-4o-mini"
	defaultAPIURL := "https://api.openai.com/v1"
	defaultHistoryFile := ".promptline_conversation_history"
	defaultCommandHistoryFile := ".promptline_history"
	defaultHistoryMax := 100
	defaultToolLimits := ToolLimits{
		MaxFileSizeBytes:    tools.DefaultLimits().MaxFileSizeBytes,
		MaxDirectoryDepth:   tools.DefaultLimits().MaxDirectoryDepth,
		MaxDirectoryEntries: tools.DefaultLimits().MaxDirectoryEntries,
	}
	defaultToolRateLimits := ToolRateLimits{
		DefaultPerMinute: tools.DefaultRateLimitConfig().DefaultPerMinute,
		CooldownSeconds: map[string]int{
			"execute_shell_command": int(tools.DefaultRateLimitConfig().Cooldowns["execute_shell_command"].Seconds()),
		},
	}
	defaultToolTimeouts := ToolTimeouts{
		PerToolSeconds: map[string]int{
			"execute_shell_command": int(tools.DefaultTimeoutConfig().PerTool["execute_shell_command"].Seconds()),
		},
	}
	defaultToolOutputFilters := ToolOutputFilters{
		MaxChars:     tools.DefaultOutputFilterConfig().MaxChars,
		StripANSI:    tools.DefaultOutputFilterConfig().StripANSI,
		StripControl: tools.DefaultOutputFilterConfig().StripControl,
	}
	return &Config{
		Model:              defaultModel,
		APIURL:             defaultAPIURL,
		ToolLimits:         defaultToolLimits,
		ToolRateLimits:     defaultToolRateLimits,
		ToolTimeouts:       defaultToolTimeouts,
		ToolOutputFilters:  defaultToolOutputFilters,
		HistoryFile:        defaultHistoryFile,
		CommandHistoryFile: defaultCommandHistoryFile,
		HistoryMaxMessages: defaultHistoryMax,
	}
}

// LoadConfig loads configuration from a JSON file, applies env overrides, and validates required fields.
func LoadConfig(filepath string) (*Config, error) {
	config := DefaultConfig()

	// If config file exists, load it
	if _, err := os.Stat(filepath); err == nil {
		data, err := os.ReadFile(filepath)
		if err != nil {
			return nil, err
		}
		normalized, err := normalizeConfigJSON(data)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(normalized, config); err != nil {
			return nil, err
		}
	}

	// Env overrides (apply regardless of whether config file exists)
	// Check OPENAI_API_KEY first, then DASHSCOPE_API_KEY
	if val := os.Getenv("OPENAI_API_KEY"); val != "" {
		config.APIKey = val
	} else if val := os.Getenv("DASHSCOPE_API_KEY"); val != "" {
		config.APIKey = val
		// For DashScope, set provider-specific defaults if not already set
		if config.APIURL == "https://api.openai.com/v1" {
			config.APIURL = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
		}
	}

	if val := os.Getenv("OPENAI_API_URL"); val != "" {
		config.APIURL = val
	}

	// Set defaults for any missing values
	if config.Model == "" {
		config.Model = "gpt-4o-mini"
	}

	if config.APIURL == "" {
		config.APIURL = "https://api.openai.com/v1"
	}

	// Validation
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required (set api_key in config.json or OPENAI_API_KEY/DASHSCOPE_API_KEY)")
	}

	return config, nil
}

// ToolPolicy converts config settings into a tool policy.
func (c *Config) ToolPolicy() tools.Policy {
	policy := tools.Policy{}
	if c.Tools.Allow != nil {
		allow := make(map[string]bool, len(c.Tools.Allow))
		for _, name := range c.Tools.Allow {
			allow[name] = true
		}
		policy.Allow = allow
	}
	askList := append([]string{}, c.Tools.Ask...)
	if len(c.Tools.RequireConfirmation) > 0 {
		askList = append(askList, c.Tools.RequireConfirmation...)
	}
	if askList != nil {
		ask := make(map[string]bool, len(askList))
		for _, name := range askList {
			ask[name] = true
		}
		policy.Ask = ask
	}
	if c.Tools.Deny != nil {
		deny := make(map[string]bool, len(c.Tools.Deny))
		for _, name := range c.Tools.Deny {
			deny[name] = true
		}
		policy.Deny = deny
	}
	return policy
}

// ToolLimitsConfig returns tool limits for runtime enforcement.
func (c *Config) ToolLimitsConfig() tools.Limits {
	return tools.Limits{
		MaxFileSizeBytes:    c.ToolLimits.MaxFileSizeBytes,
		MaxDirectoryDepth:   c.ToolLimits.MaxDirectoryDepth,
		MaxDirectoryEntries: c.ToolLimits.MaxDirectoryEntries,
	}
}

// ToolPathWhitelistConfig returns the optional tool base directory whitelist.
func (c *Config) ToolPathWhitelistConfig() []string {
	return append([]string{}, c.ToolPathWhitelist...)
}

// ToolRateLimitsConfig returns rate limiting configuration for tools.
func (c *Config) ToolRateLimitsConfig() tools.RateLimitConfig {
	cooldowns := make(map[string]time.Duration, len(c.ToolRateLimits.CooldownSeconds))
	for name, seconds := range c.ToolRateLimits.CooldownSeconds {
		if seconds <= 0 {
			continue
		}
		cooldowns[name] = time.Duration(seconds) * time.Second
	}
	perTool := make(map[string]int, len(c.ToolRateLimits.PerTool))
	for name, rate := range c.ToolRateLimits.PerTool {
		perTool[name] = rate
	}

	return tools.RateLimitConfig{
		DefaultPerMinute: c.ToolRateLimits.DefaultPerMinute,
		PerTool:          perTool,
		Cooldowns:        cooldowns,
	}
}

// ToolTimeoutsConfig returns timeout configuration for tools.
func (c *Config) ToolTimeoutsConfig() tools.TimeoutConfig {
	perTool := make(map[string]time.Duration, len(c.ToolTimeouts.PerToolSeconds))
	for name, seconds := range c.ToolTimeouts.PerToolSeconds {
		if seconds <= 0 {
			continue
		}
		perTool[name] = time.Duration(seconds) * time.Second
	}

	var defaultTimeout time.Duration
	if c.ToolTimeouts.DefaultSeconds > 0 {
		defaultTimeout = time.Duration(c.ToolTimeouts.DefaultSeconds) * time.Second
	}

	return tools.TimeoutConfig{
		Default: defaultTimeout,
		PerTool: perTool,
	}
}

// ToolOutputFiltersConfig returns output filter configuration for tools.
func (c *Config) ToolOutputFiltersConfig() tools.OutputFilterConfig {
	return tools.OutputFilterConfig{
		MaxChars:     c.ToolOutputFilters.MaxChars,
		StripANSI:    c.ToolOutputFilters.StripANSI,
		StripControl: c.ToolOutputFilters.StripControl,
	}
}

// ValidationWarning represents a non-fatal configuration issue
type ValidationWarning struct {
	Field   string
	Message string
}

// Validate checks the configuration for common issues and returns warnings
func (c *Config) Validate(registry *tools.Registry) []ValidationWarning {
	var warnings []ValidationWarning

	// Validate temperature range (OpenAI expects 0-2)
	if c.Temperature != nil {
		temp := *c.Temperature
		if temp < 0 || temp > 2 {
			warnings = append(warnings, ValidationWarning{
				Field:   "temperature",
				Message: fmt.Sprintf("temperature %.2f is outside recommended range [0, 2]", temp),
			})
		}
	}

	// Validate max_tokens (OpenAI models have different limits)
	if c.MaxTokens != nil {
		tokens := *c.MaxTokens
		if tokens <= 0 {
			warnings = append(warnings, ValidationWarning{
				Field:   "max_tokens",
				Message: fmt.Sprintf("max_tokens %d must be positive", tokens),
			})
		}
		if tokens > 128000 {
			warnings = append(warnings, ValidationWarning{
				Field:   "max_tokens",
				Message: fmt.Sprintf("max_tokens %d exceeds typical model limits", tokens),
			})
		}
	}

	// Validate tool policy against registered tools
	if registry != nil {
		registeredTools := make(map[string]bool)
		for _, tool := range registry.GetTools() {
			registeredTools[tool.Name()] = true
		}

		for _, toolName := range c.Tools.Allow {
			if !registeredTools[toolName] {
				warnings = append(warnings, ValidationWarning{
					Field:   "tools.allow",
					Message: fmt.Sprintf("tool %q in allow list is not registered", toolName),
				})
			}
		}

		for _, toolName := range c.Tools.Ask {
			if !registeredTools[toolName] {
				warnings = append(warnings, ValidationWarning{
					Field:   "tools.ask",
					Message: fmt.Sprintf("tool %q in ask list is not registered", toolName),
				})
			}
		}

		for _, toolName := range c.Tools.RequireConfirmation {
			if !registeredTools[toolName] {
				warnings = append(warnings, ValidationWarning{
					Field:   "tools.require_confirmation",
					Message: fmt.Sprintf("tool %q in require_confirmation list is not registered", toolName),
				})
			}
		}

		for _, toolName := range c.Tools.Deny {
			if !registeredTools[toolName] {
				warnings = append(warnings, ValidationWarning{
					Field:   "tools.deny",
					Message: fmt.Sprintf("tool %q in deny list is not registered", toolName),
				})
			}
		}
	}

	// Validate history_max_messages
	if c.HistoryMaxMessages <= 0 {
		warnings = append(warnings, ValidationWarning{
			Field:   "history_max_messages",
			Message: fmt.Sprintf("history_max_messages %d should be positive, using default", c.HistoryMaxMessages),
		})
	}

	return warnings
}
