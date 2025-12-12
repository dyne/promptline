package theme

import (
	"errors"
	"fmt"
	"regexp"
)

// Common validation errors
var (
	ErrInvalidColor  = errors.New("invalid color format")
	ErrEmptyColor    = errors.New("color cannot be empty")
	ErrInvalidScheme = errors.New("invalid color scheme")
)

// hexColorRegex matches valid hex color codes (#RGB or #RRGGBB)
var hexColorRegex = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// ValidateTheme validates all theme color values.
func ValidateTheme(t *Theme) error {
	if t == nil {
		return fmt.Errorf("theme is nil")
	}
	
	fields := map[string]string{
		"header_text_color":        t.HeaderTextColor,
		"chat_user_color":          t.ChatUserColor,
		"chat_assistant_color":     t.ChatAssistantColor,
		"chat_error_color":         t.ChatErrorColor,
		"chat_success_color":       t.ChatSuccessColor,
		"progress_indicator_color": t.ProgressIndicatorColor,
		"input_label_color":        t.InputLabelColor,
		"input_text_color":         t.InputTextColor,
		"input_background_color":   t.InputBackgroundColor,
		"border_color":             t.BorderColor,
	}
	
	for name, value := range fields {
		if err := ValidateColor(value); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	
	return nil
}

// ValidateColor validates a single color value (hex format).
func ValidateColor(color string) error {
	if color == "" {
		return ErrEmptyColor
	}
	
	if !hexColorRegex.MatchString(color) {
		return fmt.Errorf("%w: %q (expected #RGB or #RRGGBB)", ErrInvalidColor, color)
	}
	
	return nil
}
