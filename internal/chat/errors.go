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

package chat

import (
	"fmt"

	apperrors "promptline/internal/errors"
)

// NewStreamError wraps a streaming operation error with a code and message.
func NewStreamError(operation string, err error) *apperrors.Error {
	return apperrors.Wrap(apperrors.CodeStream, fmt.Sprintf("streaming error during %s", operation), err)
}

// NewToolExecutionError wraps a tool execution error with a code and message.
func NewToolExecutionError(toolName string, err error) *apperrors.Error {
	return apperrors.Wrap(apperrors.CodeToolExecution, fmt.Sprintf("tool execution error for %s", toolName), err)
}

// NewAPIError wraps an API operation error with a code and message.
func NewAPIError(operation string, err error) *apperrors.Error {
	return apperrors.Wrap(apperrors.CodeAPI, fmt.Sprintf("API error during %s", operation), err)
}

// NewHistoryError wraps a history operation error with a code and message.
func NewHistoryError(operation, filepath string, err error) *apperrors.Error {
	return apperrors.Wrap(apperrors.CodeHistory, fmt.Sprintf("history error during %s on %s", operation, filepath), err)
}
