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

package tools

import (
	"regexp"
	"strings"
	"sync"
)

// OutputFilterConfig controls sanitization and truncation for tool outputs.
type OutputFilterConfig struct {
	MaxChars     int
	StripANSI    bool
	StripControl bool
}

const defaultMaxOutputChars = 4000

var (
	outputFiltersMu sync.RWMutex
	outputFilters   = DefaultOutputFilterConfig()
	ansiPattern     = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\x1b\][^\x1b]*(?:\x07|\x1b\\)`)
)

// DefaultOutputFilterConfig returns default output filtering settings.
func DefaultOutputFilterConfig() OutputFilterConfig {
	return OutputFilterConfig{
		MaxChars:     defaultMaxOutputChars,
		StripANSI:    true,
		StripControl: true,
	}
}

// ConfigureOutputFilters updates output sanitization settings.
func ConfigureOutputFilters(config OutputFilterConfig) {
	outputFiltersMu.Lock()
	defer outputFiltersMu.Unlock()
	outputFilters = normalizeOutputFilterConfig(config)
}

func getOutputFilters() OutputFilterConfig {
	outputFiltersMu.RLock()
	defer outputFiltersMu.RUnlock()
	return outputFilters
}

func normalizeOutputFilterConfig(config OutputFilterConfig) OutputFilterConfig {
	if config.MaxChars <= 0 {
		config.MaxChars = defaultMaxOutputChars
	}
	return config
}

func sanitizeToolOutput(output string) (string, bool) {
	config := getOutputFilters()
	sanitized := output
	if config.StripANSI {
		sanitized = ansiPattern.ReplaceAllString(sanitized, "")
	}
	if config.StripControl {
		sanitized = stripControlChars(sanitized)
	}
	return truncateString(sanitized, config.MaxChars)
}

func stripControlChars(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))
	for _, r := range input {
		if r == '\n' || r == '\r' || r == '\t' {
			builder.WriteRune(r)
			continue
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func truncateString(input string, max int) (string, bool) {
	if max <= 0 {
		return input, false
	}
	if len(input) <= max {
		return input, false
	}
	runes := []rune(input)
	if len(runes) <= max {
		return input, false
	}
	return string(runes[:max]), true
}
