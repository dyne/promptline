package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config represents the application configuration
type Config struct {
	APIKey      string   `json:"api_key"`
	APIURL      string   `json:"api_url,omitempty"`
	Model       string   `json:"model"`
	Temperature *float32 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	defaultModel := "gpt-4o-mini"
	defaultAPIURL := "https://api.openai.com/v1"
	return &Config{
		Model:  defaultModel,
		APIURL: defaultAPIURL,
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
