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
	"context"
	"testing"

	"github.com/chzyer/readline"
)

func TestOperationCanceler(t *testing.T) {
	canceler := &operationCanceler{}
	if canceler.Cancel() {
		t.Fatal("expected no cancel when unset")
	}

	ctx, cancel := context.WithCancel(context.Background())
	canceler.Set(cancel)
	if !canceler.Cancel() {
		t.Fatal("expected cancel to return true")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("expected context to be canceled")
	}

	canceler.Clear()
	if canceler.Cancel() {
		t.Fatal("expected no cancel after clear")
	}
}

func TestFilterInterruptRune(t *testing.T) {
	r, ok := filterInterruptRune(readline.CharBell)
	if ok {
		t.Fatal("expected ctrl+g to be ignored")
	}
	if r != 0 {
		t.Fatalf("expected rune to be zero when ignored, got %v", r)
	}

	r, ok = filterInterruptRune('a')
	if !ok {
		t.Fatal("expected rune to be processed")
	}
	if r != 'a' {
		t.Fatalf("expected rune to remain unchanged, got %v", r)
	}
	r, ok = filterInterruptRune(readline.CharInterrupt)
	if !ok {
		t.Fatal("expected ctrl+c to be processed")
	}
	if r != readline.CharInterrupt {
		t.Fatalf("expected ctrl+c to remain unchanged, got %v", r)
	}
}
