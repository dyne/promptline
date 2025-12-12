package theme

import (
	"testing"
)

func TestValidateColor(t *testing.T) {
	tests := []struct {
		name    string
		color   string
		wantErr bool
	}{
		{"valid 6-char hex", "#abcdef", false},
		{"valid 6-char hex uppercase", "#ABCDEF", false},
		{"valid 3-char hex", "#abc", false},
		{"valid 3-char hex uppercase", "#ABC", false},
		{"empty string", "", true},
		{"no hash", "abcdef", true},
		{"invalid length", "#abcd", true},
		{"invalid chars", "#xyz123", true},
		{"spaces", " #abcdef", true},
		{"trailing space", "#abcdef ", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateColor(tt.color)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateColor(%q) error = %v, wantErr %v", tt.color, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTheme(t *testing.T) {
	t.Run("valid theme", func(t *testing.T) {
		theme := DefaultTheme()
		if err := ValidateTheme(theme); err != nil {
			t.Errorf("ValidateTheme() with default theme should not error: %v", err)
		}
	})
	
	t.Run("nil theme", func(t *testing.T) {
		if err := ValidateTheme(nil); err == nil {
			t.Error("ValidateTheme(nil) should error")
		}
	})
	
	t.Run("invalid color in theme", func(t *testing.T) {
		theme := DefaultTheme()
		theme.ChatUserColor = "invalid"
		if err := ValidateTheme(theme); err == nil {
			t.Error("ValidateTheme() with invalid color should error")
		}
	})
	
	t.Run("empty color in theme", func(t *testing.T) {
		theme := DefaultTheme()
		theme.ChatAssistantColor = ""
		if err := ValidateTheme(theme); err == nil {
			t.Error("ValidateTheme() with empty color should error")
		}
	})
}
