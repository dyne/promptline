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
	"errors"
	"testing"
)

func TestStreamError(t *testing.T) {
	baseErr := errors.New("connection failed")
	err := NewStreamError("receive", baseErr)

	expected := "streaming error during receive: connection failed"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}

	if !errors.Is(err, baseErr) {
		t.Error("errors.Is should unwrap to base error")
	}
}

func TestToolExecutionError(t *testing.T) {
	baseErr := errors.New("file not found")
	err := NewToolExecutionError("read_file", baseErr)

	expected := "tool execution error for read_file: file not found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}

	if !errors.Is(err, baseErr) {
		t.Error("errors.Is should unwrap to base error")
	}
}

func TestAPIError(t *testing.T) {
	baseErr := errors.New("rate limit exceeded")
	err := NewAPIError("create_completion", baseErr)

	expected := "API error during create_completion: rate limit exceeded"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}

	if !errors.Is(err, baseErr) {
		t.Error("errors.Is should unwrap to base error")
	}
}

func TestHistoryError(t *testing.T) {
	baseErr := errors.New("permission denied")
	err := NewHistoryError("open", "/tmp/test.json", baseErr)

	expected := "history error during open on /tmp/test.json: permission denied"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}

	if !errors.Is(err, baseErr) {
		t.Error("errors.Is should unwrap to base error")
	}
}
