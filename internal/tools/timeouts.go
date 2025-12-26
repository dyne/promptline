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

import "time"

// TimeoutConfig configures per-tool execution timeouts.
type TimeoutConfig struct {
	Default time.Duration
	PerTool map[string]time.Duration
}

// DefaultTimeoutConfig returns the default timeout configuration.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		PerTool: map[string]time.Duration{
			"execute_shell_command": 5 * time.Second,
		},
	}
}

// TimeoutForTool returns the timeout for a tool, if configured.
func (t TimeoutConfig) TimeoutForTool(name string) time.Duration {
	if t.PerTool != nil {
		if timeout, ok := t.PerTool[name]; ok {
			return timeout
		}
	}
	return t.Default
}
