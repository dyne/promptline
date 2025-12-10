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
