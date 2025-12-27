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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFileExactMatch(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Allow: map[string]bool{
			"edit_file": true,
		},
	})

	absDir, relDir := tempDirInCwd(t)
	absPath := filepath.Join(absDir, "note.txt")
	relPath := filepath.Join(relDir, "note.txt")
	if err := os.WriteFile(absPath, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	edits := `<<<<<<< SEARCH
world
=======
there
>>>>>>> REPLACE`
	result := registry.Execute("edit_file", map[string]interface{}{
		"path":  relPath,
		"edits": edits,
	})
	if result.Error != nil {
		t.Fatalf("expected edit_file success, got: %v", result.Error)
	}

	updated, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}
	if string(updated) != "hello\nthere\n" {
		t.Fatalf("unexpected content: %q", string(updated))
	}
}

func TestEditFileWhitespaceInsensitiveMatch(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Allow: map[string]bool{
			"edit_file": true,
		},
	})

	absDir, relDir := tempDirInCwd(t)
	absPath := filepath.Join(absDir, "block.txt")
	relPath := filepath.Join(relDir, "block.txt")
	content := "    if err != nil {\n        return err\n    }\n"
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	edits := `<<<<<<< SEARCH
  if err != nil {
    return err
  }
=======
  if err != nil {
    return fmt.Errorf("wrapped: %w", err)
  }
>>>>>>> REPLACE`
	result := registry.Execute("edit_file", map[string]interface{}{
		"path":  relPath,
		"edits": edits,
	})
	if result.Error != nil {
		t.Fatalf("expected edit_file success, got: %v", result.Error)
	}

	updated, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}
	if !strings.Contains(string(updated), "wrapped: %w") {
		t.Fatalf("expected replacement to be applied, got %q", string(updated))
	}
}

func TestEditFileAcceptsMarkerVariants(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Allow: map[string]bool{
			"edit_file": true,
		},
	})

	absDir, relDir := tempDirInCwd(t)
	absPath := filepath.Join(absDir, "note.txt")
	relPath := filepath.Join(relDir, "note.txt")

	tests := []string{
		`<<<<<<  SEARCH
world
=======
there
>>>>>>>>   REPLACE`,
		`<<<<<<<<   SEARCH
world
=======
there
>>>>>>  REPLACE`,
	}

	for _, edits := range tests {
		if err := os.WriteFile(absPath, []byte("hello\nworld\n"), 0o644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		result := registry.Execute("edit_file", map[string]interface{}{
			"path":  relPath,
			"edits": edits,
		})
		if result.Error != nil {
			t.Fatalf("expected edit_file success, got: %v", result.Error)
		}

		updated, err := os.ReadFile(absPath)
		if err != nil {
			t.Fatalf("failed to read updated file: %v", err)
		}
		if string(updated) != "hello\nthere\n" {
			t.Fatalf("unexpected content: %q", string(updated))
		}
	}
}

func TestEditFileFuzzyMatch(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Allow: map[string]bool{
			"edit_file": true,
		},
	})

	absDir, relDir := tempDirInCwd(t)
	absPath := filepath.Join(absDir, "fuzzy.txt")
	relPath := filepath.Join(relDir, "fuzzy.txt")
	if err := os.WriteFile(absPath, []byte("return err\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	edits := `<<<<<<< SEARCH
return erro
=======
return fmt.Errorf("oops: %w", err)
>>>>>>> REPLACE`
	result := registry.Execute("edit_file", map[string]interface{}{
		"path":  relPath,
		"edits": edits,
	})
	if result.Error != nil {
		t.Fatalf("expected edit_file success, got: %v", result.Error)
	}

	updated, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}
	if !strings.Contains(string(updated), "oops: %w") {
		t.Fatalf("expected fuzzy replacement to be applied, got %q", string(updated))
	}
}

func TestEditFileRejectsMultipleMatches(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Allow: map[string]bool{
			"edit_file": true,
		},
	})

	absDir, relDir := tempDirInCwd(t)
	absPath := filepath.Join(absDir, "dup.txt")
	relPath := filepath.Join(relDir, "dup.txt")
	if err := os.WriteFile(absPath, []byte("dup\ndup\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	edits := `<<<<<<< SEARCH
dup
=======
unique
>>>>>>> REPLACE`
	result := registry.Execute("edit_file", map[string]interface{}{
		"path":  relPath,
		"edits": edits,
	})
	if result.Error == nil {
		t.Fatal("expected error for multiple matches")
	}
}

func TestEditFileRejectsInvalidFormat(t *testing.T) {
	registry := NewRegistryWithPolicy(Policy{
		Allow: map[string]bool{
			"edit_file": true,
		},
	})

	absDir, relDir := tempDirInCwd(t)
	absPath := filepath.Join(absDir, "format.txt")
	relPath := filepath.Join(relDir, "format.txt")
	if err := os.WriteFile(absPath, []byte("content\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	result := registry.Execute("edit_file", map[string]interface{}{
		"path":  relPath,
		"edits": "no markers here",
	})
	if result.Error == nil {
		t.Fatal("expected format error")
	}
}
