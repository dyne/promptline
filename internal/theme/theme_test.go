package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultTheme(t *testing.T) {
	theme := DefaultTheme()
	
	if theme.HeaderTextColor == "" {
		t.Error("expected HeaderTextColor to be set")
	}
	if theme.ChatUserColor == "" {
		t.Error("expected ChatUserColor to be set")
	}
	if theme.ChatAssistantColor == "" {
		t.Error("expected ChatAssistantColor to be set")
	}
	if theme.ChatErrorColor == "" {
		t.Error("expected ChatErrorColor to be set")
	}
}

func TestLoadThemeNonExistent(t *testing.T) {
	theme, err := LoadTheme("/nonexistent/theme.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if theme == nil {
		t.Fatal("expected default theme to be returned")
	}
	// Should return defaults
	if theme.HeaderTextColor == "" {
		t.Error("expected default theme to have HeaderTextColor")
	}
}

func TestLoadThemeValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	themeFile := filepath.Join(tmpDir, "theme.json")
	
	content := `{
		"header_text_color": "#ff0000",
		"chat_user_color": "#00ff00",
		"chat_assistant_color": "#0000ff",
		"chat_error_color": "#ff00ff",
		"chat_success_color": "#ffff00",
		"progress_indicator_color": "#00ffff",
		"input_label_color": "#ffffff",
		"input_text_color": "#000000",
		"input_background_color": "#111111",
		"border_color": "#222222"
	}`
	
	if err := os.WriteFile(themeFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write theme file: %v", err)
	}
	
	theme, err := LoadTheme(themeFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if theme.HeaderTextColor != "#ff0000" {
		t.Errorf("expected HeaderTextColor=#ff0000, got %s", theme.HeaderTextColor)
	}
	if theme.ChatUserColor != "#00ff00" {
		t.Errorf("expected ChatUserColor=#00ff00, got %s", theme.ChatUserColor)
	}
}

func TestLoadThemeInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	themeFile := filepath.Join(tmpDir, "theme.json")
	
	if err := os.WriteFile(themeFile, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("failed to write theme file: %v", err)
	}
	
	_, err := LoadTheme(themeFile)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestToColorScheme(t *testing.T) {
	theme := DefaultTheme()
	colors := theme.ToColorScheme()
	
	if colors == nil {
		t.Fatal("expected ColorScheme to be returned")
	}
	if colors.Header == nil {
		t.Error("expected Header style to be set")
	}
	if colors.User == nil {
		t.Error("expected User color to be set")
	}
	if colors.Assistant == nil {
		t.Error("expected Assistant color to be set")
	}
	if colors.Error == nil {
		t.Error("expected Error color to be set")
	}
	if colors.Success == nil {
		t.Error("expected Success color to be set")
	}
	if colors.ProgressIndicator == nil {
		t.Error("expected ProgressIndicator style to be set")
	}
}

func TestDefaultColorScheme(t *testing.T) {
	colors := DefaultColorScheme()
	
	if colors == nil {
		t.Fatal("expected ColorScheme to be returned")
	}
	if colors.Header == nil {
		t.Error("expected Header style to be set")
	}
	if colors.User == nil {
		t.Error("expected User color to be set")
	}
	if colors.Assistant == nil {
		t.Error("expected Assistant color to be set")
	}
	if colors.Error == nil {
		t.Error("expected Error color to be set")
	}
	if colors.Success == nil {
		t.Error("expected Success color to be set")
	}
	if colors.ProgressIndicator == nil {
		t.Error("expected ProgressIndicator style to be set")
	}
}

func TestColorSchemeCanPrint(t *testing.T) {
	colors := DefaultColorScheme()
	
	// Just verify they don't panic when called
	// We can't easily test the actual output without capturing stdout
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ColorScheme methods panicked: %v", r)
		}
	}()
	
	_ = colors.Header.Sprint("test")
	_ = colors.User.Sprint("test")
	_ = colors.Assistant.Sprint("test")
	_ = colors.Error.Sprint("test")
	_ = colors.Success.Sprint("test")
	_ = colors.ProgressIndicator.Sprint("test")
}
