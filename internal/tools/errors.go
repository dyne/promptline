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
	"fmt"
)

// Common tool errors
var (
	// ErrToolNotAllowed indicates a tool is blocked by the current policy.
	ErrToolNotAllowed = errors.New("tool blocked by policy")

	// ErrToolRequiresConfirmation indicates a tool requires confirmation before running.
	ErrToolRequiresConfirmation = errors.New("tool requires confirmation")

	// ErrToolNotFound indicates the requested tool doesn't exist in the registry.
	ErrToolNotFound = errors.New("tool not found")

	// ErrInvalidArguments indicates tool arguments are invalid or malformed.
	ErrInvalidArguments = errors.New("invalid tool arguments")
)

// ToolExecutionError represents an error during tool execution.
type ToolExecutionError struct {
	ToolName  string
	Operation string
	Err       error
}

func (e *ToolExecutionError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("tool %s failed during %s: %v", e.ToolName, e.Operation, e.Err)
	}
	return fmt.Sprintf("tool %s failed: %v", e.ToolName, e.Err)
}

func (e *ToolExecutionError) Unwrap() error {
	return e.Err
}

// PermissionError represents an error related to tool permissions.
type PermissionError struct {
	ToolName string
	Reason   string
}

func (e *PermissionError) Error() string {
	return fmt.Sprintf("permission denied for tool %s: %s", e.ToolName, e.Reason)
}
