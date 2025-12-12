package theme

import (
	"fmt"
	"os"
)

// Manager handles theme lifecycle including loading, validation, and NO_COLOR support.
type Manager struct {
	theme       *Theme
	colorScheme *ColorScheme
	noColor     bool
}

// NewManager creates a new theme manager with the given theme file.
// It respects the NO_COLOR environment variable.
func NewManager(filepath string) (*Manager, error) {
	// Check NO_COLOR environment variable
	noColor := os.Getenv("NO_COLOR") != ""
	
	theme, err := LoadTheme(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to load theme: %w", err)
	}
	
	// Validate theme
	if err := ValidateTheme(theme); err != nil {
		return nil, fmt.Errorf("invalid theme: %w", err)
	}
	
	colorScheme := theme.ToColorScheme()
	
	// Disable colors if NO_COLOR is set
	if noColor {
		colorScheme = DisabledColorScheme()
	}
	
	return &Manager{
		theme:       theme,
		colorScheme: colorScheme,
		noColor:     noColor,
	}, nil
}

// NewManagerWithTheme creates a manager with a provided theme (for testing).
func NewManagerWithTheme(theme *Theme) *Manager {
	noColor := os.Getenv("NO_COLOR") != ""
	colorScheme := theme.ToColorScheme()
	
	if noColor {
		colorScheme = DisabledColorScheme()
	}
	
	return &Manager{
		theme:       theme,
		colorScheme: colorScheme,
		noColor:     noColor,
	}
}

// ColorScheme returns the current color scheme.
func (m *Manager) ColorScheme() *ColorScheme {
	return m.colorScheme
}

// Theme returns the current theme.
func (m *Manager) Theme() *Theme {
	return m.theme
}

// IsColorDisabled returns true if colors are disabled (NO_COLOR set).
func (m *Manager) IsColorDisabled() bool {
	return m.noColor
}

// Reload reloads the theme from the file (for runtime theme switching).
func (m *Manager) Reload(filepath string) error {
	theme, err := LoadTheme(filepath)
	if err != nil {
		return fmt.Errorf("failed to reload theme: %w", err)
	}
	
	if err := ValidateTheme(theme); err != nil {
		return fmt.Errorf("invalid theme: %w", err)
	}
	
	m.theme = theme
	m.colorScheme = theme.ToColorScheme()
	
	if m.noColor {
		m.colorScheme = DisabledColorScheme()
	}
	
	return nil
}
