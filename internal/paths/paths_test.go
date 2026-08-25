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

func TestPathConfinementTables(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "escape")); err != nil {
		t.Fatal(err)
	}
	baseResolved, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "existing descendant", path: "inside"},
		{name: "missing descendant", path: "inside/new-file"},
		{name: "dot traversal", path: "../outside", wantErr: true},
		{name: "absolute path", path: outside, wantErr: true},
		{name: "symlink escape", path: "escape/file", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveWithinBase(tc.path, base)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ResolveWithinBase(%q) error = %v, wantErr %t", tc.path, err, tc.wantErr)
			}
			if err == nil && (!filepath.IsAbs(got) || filepath.Clean(got) != got || !HasPathPrefix(got, baseResolved)) {
				t.Fatalf("resolved path = %q, want clean absolute descendant of %q", got, baseResolved)
			}
		})
	}
}

func TestValidatePathStringTables(t *testing.T) {
	for _, tc := range []struct {
		name    string
		path    string
		maxLen  int
		wantErr bool
	}{
		{name: "ordinary", path: "directory/file", maxLen: 32},
		{name: "empty", path: "", wantErr: true},
		{name: "spaces", path: " \t", wantErr: true},
		{name: "nul", path: "bad\x00path", wantErr: true},
		{name: "invalid utf8", path: string([]byte{0xff}), wantErr: true},
		{name: "combining mark", path: "e\u0301", wantErr: true},
		{name: "length expansion", path: "a/../long", maxLen: 3, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePathString(tc.path, tc.maxLen); (err != nil) != tc.wantErr {
				t.Fatalf("ValidatePathString(%q) error = %v, wantErr %t", tc.path, err, tc.wantErr)
			}
		})
	}
}

func TestHasPathPrefixRejectsSiblingPrefixConfusion(t *testing.T) {
	base := filepath.Join(t.TempDir(), "base")
	if HasPathPrefix(base+"-sibling", base) {
		t.Fatal("sibling with a shared string prefix was accepted")
	}
	if !HasPathPrefix(filepath.Join(base, "child"), base) {
		t.Fatal("descendant was rejected")
	}
}

func TestResolveWhitelistEntry(t *testing.T) {
	base := t.TempDir()
	inside := filepath.Join(base, "inside")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		entry string
		want  string
	}{
		{name: "relative existing", entry: "inside", want: inside},
		{name: "relative missing", entry: "new", want: filepath.Join(base, "new")},
		{name: "absolute existing", entry: inside, want: inside},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveWhitelistEntry(tc.entry, base)
			if err != nil || got != tc.want {
				t.Fatalf("ResolveWhitelistEntry(%q) = %q, %v; want %q", tc.entry, got, err, tc.want)
			}
		})
	}
}

func FuzzPathConfinement(f *testing.F) {
	for _, seed := range []string{"inside", "inside/new", "../escape", "", ".", "a/../../b", "e\u0301"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		base := t.TempDir()
		if err := os.Mkdir(filepath.Join(base, "inside"), 0o755); err != nil {
			t.Fatal(err)
		}
		baseResolved, err := filepath.EvalSymlinks(base)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidatePathString(input, 512); err == nil {
			got, err := ResolveWithinBase(input, base)
			if err == nil && (!filepath.IsAbs(got) || filepath.Clean(got) != got || !HasPathPrefix(got, baseResolved)) {
				t.Fatalf("unsafe result %q for input %q", got, input)
			}
		}
	})
}
