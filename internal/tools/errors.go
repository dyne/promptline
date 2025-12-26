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

	apperrors "promptline/internal/errors"
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
)

// NewToolExecutionError wraps a tool execution error with a shared error code.
func NewToolExecutionError(toolName, operation string, err error) *apperrors.Error {
	if operation != "" {
		return apperrors.Wrap(apperrors.CodeToolExecution, fmt.Sprintf("tool %s failed during %s", toolName, operation), err)
	}
	return apperrors.Wrap(apperrors.CodeToolExecution, fmt.Sprintf("tool %s failed", toolName), err)
}

// NewPermissionError wraps a permission error with a shared error code.
func NewPermissionError(toolName, reason string) *apperrors.Error {
	return apperrors.New(apperrors.CodePermission, fmt.Sprintf("permission denied for tool %s: %s", toolName, reason))
}
