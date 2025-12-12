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
	"encoding/json"
	"os"

	"github.com/fatih/color"
	"github.com/pterm/pterm"
)

// Theme represents the color theme for the streaming console
type Theme struct {
	HeaderTextColor        string `json:"header_text_color"`
	ChatUserColor          string `json:"chat_user_color"`
	ChatAssistantColor     string `json:"chat_assistant_color"`
	ChatErrorColor         string `json:"chat_error_color"`
	ChatSuccessColor       string `json:"chat_success_color"`
	ProgressIndicatorColor string `json:"progress_indicator_color"`
	InputLabelColor        string `json:"input_label_color"`
	InputTextColor         string `json:"input_text_color"`
	InputBackgroundColor   string `json:"input_background_color"`
	BorderColor            string `json:"border_color"`
}

// ColorScheme provides pterm and color styles based on theme
type ColorScheme struct {
	Header             *pterm.Style
	User               *color.Color
	Assistant          *color.Color
	Error              *color.Color
	Success            *color.Color
	ProgressIndicator  *pterm.Style
}

// DefaultTheme returns a theme with default values
func DefaultTheme() *Theme {
	return &Theme{
		HeaderTextColor:        "#cba6f7",
		ChatUserColor:          "#89b4fa",
		ChatAssistantColor:     "#a6e3a1",
		ChatErrorColor:         "#f38ba8",
		ChatSuccessColor:       "#a6e3a1",
		ProgressIndicatorColor: "#fab387",
		InputLabelColor:        "#cdd6f4",
		InputTextColor:         "#cdd6f4",
		InputBackgroundColor:   "#1e1e2e",
		BorderColor:            "#6c7086",
	}
}

// LoadTheme loads theme configuration from a JSON file
func LoadTheme(filepath string) (*Theme, error) {
	theme := DefaultTheme()

	// If theme file doesn't exist, return default theme
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return theme, nil
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, theme); err != nil {
		return nil, err
	}

	return theme, nil
}

// ToColorScheme converts theme to pterm/color styles
func (t *Theme) ToColorScheme() *ColorScheme {
	return &ColorScheme{
		Header:            pterm.NewStyle(pterm.FgLightMagenta),
		User:              color.New(color.FgCyan),
		Assistant:         color.New(color.FgGreen),
		Error:             color.New(color.FgRed),
		Success:           color.New(color.FgGreen),
		ProgressIndicator: pterm.NewStyle(pterm.FgYellow),
	}
}

// DefaultColorScheme returns a simple color scheme using pterm defaults
func DefaultColorScheme() *ColorScheme {
	return &ColorScheme{
		Header:            pterm.NewStyle(pterm.FgCyan, pterm.Bold),
		User:              color.New(color.FgBlue),
		Assistant:         color.New(color.FgGreen),
		Error:             color.New(color.FgRed, color.Bold),
		Success:           color.New(color.FgGreen),
		ProgressIndicator: pterm.NewStyle(pterm.FgYellow),
	}
}

// DisabledColorScheme returns a color scheme with all colors disabled (for NO_COLOR).
func DisabledColorScheme() *ColorScheme {
	// Disable color output for fatih/color
	color.NoColor = true
	
	return &ColorScheme{
		Header:            pterm.NewStyle(), // No colors
		User:              color.New(),      // No colors
		Assistant:         color.New(),
		Error:             color.New(),
		Success:           color.New(),
		ProgressIndicator: pterm.NewStyle(),
	}
}