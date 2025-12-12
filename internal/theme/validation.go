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
