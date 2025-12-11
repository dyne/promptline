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
	// Missing file returns default config immediately (without env override or validation)
	// This is current behavior - returns early at line 42
	cfg, err := LoadConfig("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected default config to be returned")
	}
	// Default config has no API key, but validation is skipped for missing files
	if cfg.Model == "" {
		t.Error("expected default model to be set")
	}
	if cfg.APIURL == "" {
		t.Error("expected default API URL to be set")
	}
}
