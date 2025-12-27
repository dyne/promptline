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

package main

import (
	"errors"
	"io"
	"testing"

	"github.com/chzyer/readline"
)

func TestClassifyReadlineError(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		err      error
		expected readlineAction
	}{
		{"interrupt", "", readline.ErrInterrupt, readlineContinue},
		{"eof-empty", "", io.EOF, readlineExit},
		{"eof-whitespace", "   ", io.EOF, readlineExit},
		{"eof-line", "hello", io.EOF, readlineContinue},
		{"other", "", errors.New("boom"), readlineUnhandled},
	}

	for _, tc := range cases {
		if got := classifyReadlineError(tc.line, tc.err); got != tc.expected {
			t.Fatalf("%s: expected %v, got %v", tc.name, tc.expected, got)
		}
	}
}

func TestSanitizeInputLine(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"\x03/quit", "\x03/quit"},
		{"\x07/quit", "\x07/quit"},
		{"\x1f\t/quit", "\x1f\t/quit"},
		{"/quit", "/quit"},
	}

	for _, tc := range cases {
		if got := sanitizeInputLine(tc.input); got != tc.expected {
			t.Fatalf("expected %q, got %q", tc.expected, got)
		}
	}
}
