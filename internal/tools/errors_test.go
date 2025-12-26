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
	"errors"
	"testing"
)

func TestToolExecutionError(t *testing.T) {
	baseErr := errors.New("execution failed")

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "with operation",
			err:      NewToolExecutionError("execute_shell_command", "run", baseErr),
			expected: "tool execute_shell_command failed during run: execution failed",
		},
		{
			name:     "without operation",
			err:      NewToolExecutionError("read_file", "", baseErr),
			expected: "tool read_file failed: execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.err.Error())
			}

			if !errors.Is(tt.err, baseErr) {
				t.Error("errors.Is should unwrap to base error")
			}
		})
	}
}

func TestPermissionError(t *testing.T) {
	err := NewPermissionError("write_file", "tool requires confirmation")

	expected := "permission denied for tool write_file: tool requires confirmation"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "ErrToolNotAllowed",
			err:  ErrToolNotAllowed,
			msg:  "tool blocked by policy",
		},
		{
			name: "ErrToolRequiresConfirmation",
			err:  ErrToolRequiresConfirmation,
			msg:  "tool requires confirmation",
		},
		{
			name: "ErrToolNotFound",
			err:  ErrToolNotFound,
			msg:  "tool not found",
		},
		{
			name: "ErrInvalidArguments",
			err:  ErrInvalidArguments,
			msg:  "invalid tool arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("expected %q, got %q", tt.msg, tt.err.Error())
			}
		})
	}
}
