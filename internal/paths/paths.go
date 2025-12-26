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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidatePathString validates raw path input before resolution.
func ValidatePathString(path string, maxLen int) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if strings.IndexByte(path, 0) != -1 {
		return fmt.Errorf("path contains null byte")
	}
	if !utf8.ValidString(path) {
		return fmt.Errorf("path is not valid UTF-8")
	}
	for _, r := range path {
		if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r) {
			return fmt.Errorf("path contains unsupported unicode combining mark")
		}
	}
	if maxLen > 0 {
		if len(path) > maxLen {
			return fmt.Errorf("path exceeds maximum length of %d characters", maxLen)
		}
		if len(filepath.Clean(path)) > maxLen {
			return fmt.Errorf("path exceeds maximum length of %d characters", maxLen)
		}
	}
	return nil
}

// ResolveWithinBase resolves a relative path under a base directory.
func ResolveWithinBase(path, baseDir string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %v", err)
	}
	baseResolved, err := filepath.EvalSymlinks(baseAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %v", err)
	}

	cleanRel := filepath.Clean(path)
	absPath := filepath.Clean(filepath.Join(baseResolved, cleanRel))
	if !HasPathPrefix(absPath, baseResolved) {
		return "", fmt.Errorf("path escapes working directory")
	}

	resolved, err := ResolveSymlinkedPath(absPath, baseResolved)
	if err != nil {
		return "", err
	}

	if !HasPathPrefix(resolved, baseResolved) {
		return "", fmt.Errorf("path escapes working directory")
	}

	return resolved, nil
}

// ResolveSymlinkedPath resolves symlinks while ensuring the base stays within bounds.
func ResolveSymlinkedPath(path, baseResolved string) (string, error) {
	if _, err := os.Lstat(path); err == nil {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve path: %v", err)
		}
		return resolved, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to stat path: %v", err)
	}

	parent := filepath.Dir(path)
	parentResolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", fmt.Errorf("failed to resolve parent path: %v", err)
	}
	if !HasPathPrefix(parentResolved, baseResolved) {
		return "", fmt.Errorf("path escapes working directory")
	}
	return filepath.Join(parentResolved, filepath.Base(path)), nil
}

// HasPathPrefix returns true when path is within base.
func HasPathPrefix(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
}

// ResolveWhitelistEntry resolves a whitelist entry relative to a base.
func ResolveWhitelistEntry(entry, baseResolved string) (string, error) {
	candidate := entry
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(baseResolved, candidate)
	}
	candidate = filepath.Clean(candidate)
	if _, err := os.Lstat(candidate); err == nil {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return "", fmt.Errorf("failed to resolve allowed path: %v", err)
		}
		return resolved, nil
	} else if os.IsNotExist(err) {
		return candidate, nil
	} else {
		return "", fmt.Errorf("failed to stat allowed path: %v", err)
	}
}
