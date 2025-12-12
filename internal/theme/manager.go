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
