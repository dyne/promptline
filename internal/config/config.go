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
	"promptline/internal/tools"
)

// Config represents the application configuration
type Config struct {
	APIKey             string       `json:"api_key"`
	APIURL             string       `json:"api_url,omitempty"`
	Model              string       `json:"model"`
	Temperature        *float32     `json:"temperature,omitempty"`
	MaxTokens          *int         `json:"max_tokens,omitempty"`
	Tools              ToolSettings `json:"tools,omitempty"`
	HistoryFile        string       `json:"history_file,omitempty"`
	CommandHistoryFile string       `json:"command_history_file,omitempty"`
	HistoryMaxMessages int          `json:"history_max_messages,omitempty"`
	Sandbox            Sandbox      `json:"sandbox,omitempty"`
}

// ToolSettings describes tool allow/confirm lists.
type ToolSettings struct {
	Allow               []string `json:"allow"`
	RequireConfirmation []string `json:"require_confirmation"`
}

// Sandbox describes filesystem sandbox settings.
type Sandbox struct {
	Enabled       bool     `json:"enabled"`
	Workdir       string   `json:"workdir,omitempty"`
	ReadOnlyPaths []string `json:"read_only_paths,omitempty"`
	MaskedPaths   []string `json:"masked_paths,omitempty"`
	NonRootUser   bool     `json:"non_root_user,omitempty"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	defaultModel := "gpt-4o-mini"
	defaultAPIURL := "https://api.openai.com/v1"
	defaultHistoryFile := ".promptline_conversation_history"
	defaultCommandHistoryFile := ".promptline_history"
	defaultHistoryMax := 100
	return &Config{
		Model:              defaultModel,
		APIURL:             defaultAPIURL,
		HistoryFile:        defaultHistoryFile,
		CommandHistoryFile: defaultCommandHistoryFile,
		HistoryMaxMessages: defaultHistoryMax,
		Sandbox: Sandbox{
			Enabled:     true,
			NonRootUser: true,
		},
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

		if err := json.Unmarshal(data, config); err != nil {
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
		policy.Allowed = allow
	}
	if c.Tools.RequireConfirmation != nil {
		confirm := make(map[string]bool, len(c.Tools.RequireConfirmation))
		for _, name := range c.Tools.RequireConfirmation {
			confirm[name] = true
		}
		policy.RequireConfirmation = confirm
	}
	return policy
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
			registeredTools[tool.Name] = true
		}

		for _, toolName := range c.Tools.Allow {
			if !registeredTools[toolName] {
				warnings = append(warnings, ValidationWarning{
					Field:   "tools.allow",
					Message: fmt.Sprintf("tool %q in allow list is not registered", toolName),
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
