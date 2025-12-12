package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	t.Run("with default theme", func(t *testing.T) {
		mgr, err := NewManager("nonexistent.json")
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		if mgr == nil {
			t.Fatal("expected manager to be created")
		}
		if mgr.Theme() == nil {
			t.Error("expected theme to be set")
		}
		if mgr.ColorScheme() == nil {
			t.Error("expected color scheme to be set")
		}
	})
	
	t.Run("with custom theme file", func(t *testing.T) {
		tmpDir := t.TempDir()
		themeFile := filepath.Join(tmpDir, "theme.json")
		
		themeJSON := `{
			"header_text_color": "#ffffff",
			"chat_user_color": "#00ff00",
			"chat_assistant_color": "#0000ff",
			"chat_error_color": "#ff0000",
			"chat_success_color": "#00ff00",
			"progress_indicator_color": "#ffff00",
			"input_label_color": "#ffffff",
			"input_text_color": "#ffffff",
			"input_background_color": "#000000",
			"border_color": "#888888"
		}`
		
		if err := os.WriteFile(themeFile, []byte(themeJSON), 0644); err != nil {
			t.Fatalf("failed to write theme file: %v", err)
		}
		
		mgr, err := NewManager(themeFile)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		if mgr.Theme().ChatUserColor != "#00ff00" {
			t.Errorf("expected user color #00ff00, got %s", mgr.Theme().ChatUserColor)
		}
	})
	
	t.Run("with invalid theme", func(t *testing.T) {
		tmpDir := t.TempDir()
		themeFile := filepath.Join(tmpDir, "invalid.json")
		
		invalidJSON := `{
			"header_text_color": "not-a-color"
		}`
		
		if err := os.WriteFile(themeFile, []byte(invalidJSON), 0644); err != nil {
			t.Fatalf("failed to write theme file: %v", err)
		}
		
		_, err := NewManager(themeFile)
		if err == nil {
			t.Error("NewManager() with invalid theme should error")
		}
	})
}

func TestNewManagerWithTheme(t *testing.T) {
	theme := DefaultTheme()
	mgr := NewManagerWithTheme(theme)
	
	if mgr == nil {
		t.Fatal("expected manager to be created")
	}
	if mgr.Theme() != theme {
		t.Error("expected manager to use provided theme")
	}
}

func TestManagerNOCOLOR(t *testing.T) {
	t.Run("NO_COLOR not set", func(t *testing.T) {
		os.Unsetenv("NO_COLOR")
		mgr, err := NewManager("nonexistent.json")
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		if mgr.IsColorDisabled() {
			t.Error("expected colors to be enabled")
		}
	})
	
	t.Run("NO_COLOR set", func(t *testing.T) {
		os.Setenv("NO_COLOR", "1")
		defer os.Unsetenv("NO_COLOR")
		
		mgr, err := NewManager("nonexistent.json")
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		if !mgr.IsColorDisabled() {
			t.Error("expected colors to be disabled")
		}
	})
}

func TestManagerReload(t *testing.T) {
	tmpDir := t.TempDir()
	themeFile := filepath.Join(tmpDir, "theme.json")
	
	// Initial theme
	themeJSON := `{
		"header_text_color": "#ffffff",
		"chat_user_color": "#00ff00",
		"chat_assistant_color": "#0000ff",
		"chat_error_color": "#ff0000",
		"chat_success_color": "#00ff00",
		"progress_indicator_color": "#ffff00",
		"input_label_color": "#ffffff",
		"input_text_color": "#ffffff",
		"input_background_color": "#000000",
		"border_color": "#888888"
	}`
	
	if err := os.WriteFile(themeFile, []byte(themeJSON), 0644); err != nil {
		t.Fatalf("failed to write theme file: %v", err)
	}
	
	mgr, err := NewManager(themeFile)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	
	originalColor := mgr.Theme().ChatUserColor
	
	// Update theme file
	newThemeJSON := `{
		"header_text_color": "#ffffff",
		"chat_user_color": "#ff00ff",
		"chat_assistant_color": "#0000ff",
		"chat_error_color": "#ff0000",
		"chat_success_color": "#00ff00",
		"progress_indicator_color": "#ffff00",
		"input_label_color": "#ffffff",
		"input_text_color": "#ffffff",
		"input_background_color": "#000000",
		"border_color": "#888888"
	}`
	
	if err := os.WriteFile(themeFile, []byte(newThemeJSON), 0644); err != nil {
		t.Fatalf("failed to write updated theme file: %v", err)
	}
	
	// Reload
	if err := mgr.Reload(themeFile); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	
	newColor := mgr.Theme().ChatUserColor
	if newColor == originalColor {
		t.Error("expected color to change after reload")
	}
	if newColor != "#ff00ff" {
		t.Errorf("expected color #ff00ff, got %s", newColor)
	}
}
