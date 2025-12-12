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
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func TestEnvOverridesFile(t *testing.T) {
	path := writeTempConfig(t, `{"api_key":"file-key","model":"gpt-file","api_url":"https://file.example"}`)
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("OPENAI_API_URL", "https://env.example")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Fatalf("expected env key to override file, got %s", cfg.APIKey)
	}
	if cfg.APIURL != "https://env.example" {
		t.Fatalf("expected env api url to override file, got %s", cfg.APIURL)
	}
}

func TestMissingAPIKeyReturnsError(t *testing.T) {
	path := writeTempConfig(t, `{}`)
	// clear envs
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_URL", "")

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestDefaultsApplied(t *testing.T) {
	path := writeTempConfig(t, `{"api_key":"k"}`)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_API_URL", "")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model == "" {
		t.Fatalf("expected default model to be set")
	}
	if cfg.APIURL == "" {
		t.Fatalf("expected default api url to be set")
	}
}

func TestToolPolicyEmpty(t *testing.T) {
	path := writeTempConfig(t, `{"api_key":"test-key"}`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	policy := cfg.ToolPolicy()

	// Empty config tools should return empty maps (not nil, but empty)
	// The tool registry itself applies defaults via NewRegistry()
	if len(policy.Allowed) != 0 {
		t.Error("expected empty Allowed map when no tools configured")
	}
	if len(policy.RequireConfirmation) != 0 {
		t.Error("expected empty RequireConfirmation map when no tools configured")
	}
}

func TestCustomToolPolicy(t *testing.T) {
	content := `{
		"api_key": "test-key",
		"tools": {
			"allow": ["custom_tool"],
			"require_confirmation": ["another_tool"]
		}
	}`
	path := writeTempConfig(t, content)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	policy := cfg.ToolPolicy()

	// Custom allow list
	if !policy.Allowed["custom_tool"] {
		t.Error("expected custom_tool to be in allow list")
	}

	// Custom confirmation list
	if !policy.RequireConfirmation["another_tool"] {
		t.Error("expected another_tool to be in confirmation list")
	}
}

func TestTemperatureAndMaxTokensOptional(t *testing.T) {
	path := writeTempConfig(t, `{"api_key":"k"}`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Temperature != nil {
		t.Error("expected Temperature to be nil when not specified")
	}
	if cfg.MaxTokens != nil {
		t.Error("expected MaxTokens to be nil when not specified")
	}
}

func TestTemperatureAndMaxTokensSet(t *testing.T) {
	content := `{
		"api_key": "k",
		"temperature": 0.8,
		"max_tokens": 2000
	}`
	path := writeTempConfig(t, content)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Temperature == nil {
		t.Fatal("expected Temperature to be set")
	}
	if *cfg.Temperature != 0.8 {
		t.Errorf("expected Temperature=0.8, got %f", *cfg.Temperature)
	}

	if cfg.MaxTokens == nil {
		t.Fatal("expected MaxTokens to be set")
	}
	if *cfg.MaxTokens != 2000 {
		t.Errorf("expected MaxTokens=2000, got %d", *cfg.MaxTokens)
	}
}

func TestLoadConfigMissingFileReturnsDefault(t *testing.T) {
	// Missing file with env key should still work
	t.Setenv("OPENAI_API_KEY", "test-key")
	cfg, err := LoadConfig("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected default config to be returned")
	}
	if cfg.APIKey != "test-key" {
		t.Error("expected env API key to be applied even without config file")
	}
	if cfg.Model == "" {
		t.Error("expected default model to be set")
	}
	if cfg.APIURL == "" {
		t.Error("expected default API URL to be set")
	}
}

func TestDashScopeAPIKeyEnvVar(t *testing.T) {
	path := writeTempConfig(t, `{"model":"qwen3"}`)
	t.Setenv("DASHSCOPE_API_KEY", "dashscope-key")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "dashscope-key" {
		t.Fatalf("expected DASHSCOPE_API_KEY to be used, got %s", cfg.APIKey)
	}
	// Should set DashScope default URL
	if cfg.APIURL != "https://dashscope-intl.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("expected DashScope default URL, got %s", cfg.APIURL)
	}
}

func TestOpenAIKeyTakesPrecedenceOverDashScope(t *testing.T) {
	path := writeTempConfig(t, `{}`)
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("DASHSCOPE_API_KEY", "dashscope-key")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "openai-key" {
		t.Fatalf("expected OPENAI_API_KEY to take precedence, got %s", cfg.APIKey)
	}
	// Should use OpenAI default URL, not DashScope
	if cfg.APIURL != "https://api.openai.com/v1" {
		t.Fatalf("expected OpenAI default URL, got %s", cfg.APIURL)
	}
}

func TestDashScopeKeyWithCustomURL(t *testing.T) {
	path := writeTempConfig(t, `{"api_url":"https://custom.example/v1"}`)
	t.Setenv("DASHSCOPE_API_KEY", "dashscope-key")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Custom URL from config should be preserved
	if cfg.APIURL != "https://custom.example/v1" {
		t.Fatalf("expected custom URL to be preserved, got %s", cfg.APIURL)
	}
}

func TestMissingAPIKeyWithNoConfigFile(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("DASHSCOPE_API_KEY", "")

	_, err := LoadConfig("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing API key even without config file")
	}
}

func TestOpenAPIURLEnvOverridesDashScopeDefault(t *testing.T) {
	path := writeTempConfig(t, `{}`)
	t.Setenv("DASHSCOPE_API_KEY", "dashscope-key")
	t.Setenv("OPENAI_API_URL", "https://override.example/v1")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// OPENAI_API_URL should override even DashScope defaults
	if cfg.APIURL != "https://override.example/v1" {
		t.Fatalf("expected OPENAI_API_URL to override, got %s", cfg.APIURL)
	}
}

func TestCommandHistoryFileDefault(t *testing.T) {
	path := writeTempConfig(t, `{"api_key":"test-key"}`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CommandHistoryFile != ".promptline_history" {
		t.Fatalf("expected default command_history_file, got %s", cfg.CommandHistoryFile)
	}
}

func TestCommandHistoryFileCustom(t *testing.T) {
	path := writeTempConfig(t, `{"api_key":"test-key","command_history_file":"/tmp/custom_history"}`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CommandHistoryFile != "/tmp/custom_history" {
		t.Fatalf("expected custom command_history_file, got %s", cfg.CommandHistoryFile)
	}
}

func TestValidateTemperatureRange(t *testing.T) {
	tests := []struct {
		name          string
		temperature   *float32
		expectWarning bool
	}{
		{
			name:          "valid temperature",
			temperature:   func() *float32 { v := float32(0.7); return &v }(),
			expectWarning: false,
		},
		{
			name:          "temperature too low",
			temperature:   func() *float32 { v := float32(-0.1); return &v }(),
			expectWarning: true,
		},
		{
			name:          "temperature too high",
			temperature:   func() *float32 { v := float32(2.5); return &v }(),
			expectWarning: true,
		},
		{
			name:          "temperature at lower bound",
			temperature:   func() *float32 { v := float32(0); return &v }(),
			expectWarning: false,
		},
		{
			name:          "temperature at upper bound",
			temperature:   func() *float32 { v := float32(2); return &v }(),
			expectWarning: false,
		},
		{
			name:          "nil temperature",
			temperature:   nil,
			expectWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				APIKey:      "test-key",
				Model:       "gpt-4o-mini",
				Temperature: tt.temperature,
			}

			warnings := cfg.Validate(nil)
			hasWarning := false
			for _, w := range warnings {
				if w.Field == "temperature" {
					hasWarning = true
					break
				}
			}

			if hasWarning != tt.expectWarning {
				t.Errorf("expected warning=%v, got=%v", tt.expectWarning, hasWarning)
			}
		})
	}
}

func TestValidateMaxTokens(t *testing.T) {
	tests := []struct {
		name          string
		maxTokens     *int
		expectWarning bool
	}{
		{
			name:          "valid max tokens",
			maxTokens:     func() *int { v := 2000; return &v }(),
			expectWarning: false,
		},
		{
			name:          "negative max tokens",
			maxTokens:     func() *int { v := -100; return &v }(),
			expectWarning: true,
		},
		{
			name:          "zero max tokens",
			maxTokens:     func() *int { v := 0; return &v }(),
			expectWarning: true,
		},
		{
			name:          "excessive max tokens",
			maxTokens:     func() *int { v := 200000; return &v }(),
			expectWarning: true,
		},
		{
			name:          "nil max tokens",
			maxTokens:     nil,
			expectWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				APIKey:    "test-key",
				Model:     "gpt-4o-mini",
				MaxTokens: tt.maxTokens,
			}

			warnings := cfg.Validate(nil)
			hasWarning := false
			for _, w := range warnings {
				if w.Field == "max_tokens" {
					hasWarning = true
					break
				}
			}

			if hasWarning != tt.expectWarning {
				t.Errorf("expected warning=%v, got=%v", tt.expectWarning, hasWarning)
			}
		})
	}
}

func TestValidateHistoryMaxMessages(t *testing.T) {
	tests := []struct {
		name               string
		historyMaxMessages int
		expectWarning      bool
	}{
		{"valid positive value", 100, false},
		{"zero value", 0, true},
		{"negative value", -10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				APIKey:             "test-key",
				Model:              "gpt-4o-mini",
				HistoryMaxMessages: tt.historyMaxMessages,
			}

			warnings := cfg.Validate(nil)
			hasWarning := false
			for _, w := range warnings {
				if w.Field == "history_max_messages" {
					hasWarning = true
					break
				}
			}

			if hasWarning != tt.expectWarning {
				t.Errorf("expected warning=%v, got=%v", tt.expectWarning, hasWarning)
			}
		})
	}
}
