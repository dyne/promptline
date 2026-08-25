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

	// ErrToolDeniedByUser indicates the user denied executing a tool.
	ErrToolDeniedByUser = errors.New("tool execution denied by user")

	// ErrToolNotFound indicates the requested tool doesn't exist in the registry.
	ErrToolNotFound = errors.New("tool not found")

	// ErrInvalidArguments indicates tool arguments are invalid or malformed.
	ErrInvalidArguments = errors.New("invalid tool arguments")

	// ErrToolRateLimited indicates a tool call exceeded rate limits.
	ErrToolRateLimited = errors.New("tool rate limit exceeded")

	// ErrToolInCooldown indicates a tool is in a cooldown window.
	ErrToolInCooldown = errors.New("tool is in cooldown")

	// ErrToolCancelled indicates caller cancellation stopped a tool.
	ErrToolCancelled = errors.New("tool execution cancelled")

	// ErrToolTimeout indicates the configured deadline stopped a tool.
	ErrToolTimeout = errors.New("tool execution timed out")
)

// NewToolExecutionError adds tool context while retaining the cause.
func NewToolExecutionError(toolName, operation string, err error) error {
	if operation != "" {
		return fmt.Errorf("tool %s failed during %s: %w", toolName, operation, err)
	}
	return fmt.Errorf("tool %s failed: %w", toolName, err)
}

// NewPermissionError adds permission context without a coded wrapper.
func NewPermissionError(toolName, reason string) error {
	return fmt.Errorf("permission denied for tool %s: %s", toolName, reason)
}
