package config

import (
	"encoding/json"
	"os"
)

// Config represents the application configuration
type Config struct {
	APIKey      string  `json:"api_key"`
	BaseURL     string  `json:"base_url,omitempty"`
	Model       string  `json:"model"`
	Temperature *float32 `json:"temperature,omitempty"`
	MaxTokens   *int    `json:"max_tokens,omitempty"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	defaultModel := "gpt-4o-mini"
	defaultBaseURL := "https://api.openai.com/v1"
	return &Config{
		Model:   defaultModel,
		BaseURL: defaultBaseURL,
	}
}

// LoadConfig loads configuration from a JSON file
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

	// Set defaults for any missing values
	if config.Model == "" {
		config.Model = "gpt-4o-mini"
	}
	
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}

	return config, nil
}