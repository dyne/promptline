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

package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePathStringRejectsNullByte(t *testing.T) {
	if err := ValidatePathString("bad\x00path", 0); err == nil {
		t.Fatal("expected error for null byte path")
	}
}

func TestResolveWithinBase(t *testing.T) {
	base := t.TempDir()
	parent := filepath.Join(base, "subdir")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}
	resolved, err := ResolveWithinBase("subdir/file.txt", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	baseResolved, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatalf("failed to resolve base dir: %v", err)
	}
	if !HasPathPrefix(resolved, baseResolved) {
		t.Fatalf("expected resolved path to stay within base, got %s", resolved)
	}
}
