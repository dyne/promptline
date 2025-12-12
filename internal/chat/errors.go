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

import "fmt"

// StreamError represents an error during streaming operations.
type StreamError struct {
	Operation string
	Err       error
}

func (e *StreamError) Error() string {
	return fmt.Sprintf("streaming error during %s: %v", e.Operation, e.Err)
}

func (e *StreamError) Unwrap() error {
	return e.Err
}

// ToolExecutionError represents an error during tool execution.
type ToolExecutionError struct {
	ToolName string
	Err      error
}

func (e *ToolExecutionError) Error() string {
	return fmt.Sprintf("tool execution error for %s: %v", e.ToolName, e.Err)
}

func (e *ToolExecutionError) Unwrap() error {
	return e.Err
}

// APIError represents an error from the OpenAI API.
type APIError struct {
	Operation string
	Err       error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error during %s: %v", e.Operation, e.Err)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// HistoryError represents an error related to conversation history operations.
type HistoryError struct {
	Operation string
	Filepath  string
	Err       error
}

func (e *HistoryError) Error() string {
	return fmt.Sprintf("history error during %s on %s: %v", e.Operation, e.Filepath, e.Err)
}

func (e *HistoryError) Unwrap() error {
	return e.Err
}
