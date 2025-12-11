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
	HistoryMaxMessages int          `json:"history_max_messages,omitempty"`
}

// ToolSettings describes tool allow/confirm lists.
type ToolSettings struct {
	Allow               []string `json:"allow"`
	RequireConfirmation []string `json:"require_confirmation"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	defaultModel := "gpt-4o-mini"
	defaultAPIURL := "https://api.openai.com/v1"
	defaultHistoryFile := ".promptline_conversation_history"
	defaultHistoryMax := 100
	return &Config{
		Model:              defaultModel,
		APIURL:             defaultAPIURL,
		HistoryFile:        defaultHistoryFile,
		HistoryMaxMessages: defaultHistoryMax,
	}
}

// LoadConfig loads configuration from a JSON file, applies env overrides, and validates required fields.
func LoadConfig(filepath string) (*Config, error) {
	config := DefaultConfig()

	// If config file doesn't exist, return default config
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return config, nil
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	// Env overrides
	if val := os.Getenv("OPENAI_API_KEY"); val != "" {
		config.APIKey = val
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
