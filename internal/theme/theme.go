package theme

import (
	"encoding/json"
	"os"
)

// Theme represents the color theme for the TUI
type Theme struct {
	HeaderTextColor      string `json:"header_text_color"`
	ChatUserColor        string `json:"chat_user_color"`
	ChatAssistantColor   string `json:"chat_assistant_color"`
	ChatErrorColor       string `json:"chat_error_color"`
	ChatSuccessColor     string `json:"chat_success_color"`
	ProgressIndicatorColor string `json:"progress_indicator_color"`
	InputLabelColor      string `json:"input_label_color"`
	InputTextColor       string `json:"input_text_color"`
	InputBackgroundColor string `json:"input_background_color"`
	BorderColor          string `json:"border_color"`
}

// DefaultTheme returns a theme with default values
func DefaultTheme() *Theme {
	return &Theme{
		HeaderTextColor:      "#cba6f7",
		ChatUserColor:        "#89b4fa",
		ChatAssistantColor:   "#a6e3a1",
		ChatErrorColor:       "#f38ba8",
		ChatSuccessColor:     "#a6e3a1",
		ProgressIndicatorColor: "#fab387",
		InputLabelColor:      "#cdd6f4",
		InputTextColor:       "#cdd6f4",
		InputBackgroundColor: "#1e1e2e",
		BorderColor:          "#6c7086",
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