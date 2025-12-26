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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func setupURootBenchDir(b *testing.B) (string, func()) {
	b.Helper()
	dir, err := os.MkdirTemp(".", "promptline-uroot-bench-")
	if err != nil {
		b.Fatalf("create temp dir: %v", err)
	}
	return dir, func() {
		_ = os.RemoveAll(dir)
	}
}

func writeBenchFile(b *testing.B, path string, lines int, needleEvery int) {
	b.Helper()
	var sb strings.Builder
	for i := 0; i < lines; i++ {
		if needleEvery > 0 && i%needleEvery == 0 {
			sb.WriteString("needle ")
		}
		sb.WriteString("line ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		b.Fatalf("write bench file: %v", err)
	}
}

func populateFlatDir(b *testing.B, dir string, files int) {
	b.Helper()
	for i := 0; i < files; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file-%03d.txt", i))
		if err := os.WriteFile(path, []byte("data\n"), 0o644); err != nil {
			b.Fatalf("write flat file: %v", err)
		}
	}
}

func populateDeepDir(b *testing.B, dir string, depth int, filesPerLevel int) string {
	b.Helper()
	current := dir
	for i := 0; i < depth; i++ {
		current = filepath.Join(current, fmt.Sprintf("level-%02d", i))
		if err := os.MkdirAll(current, 0o755); err != nil {
			b.Fatalf("mkdir depth: %v", err)
		}
		for j := 0; j < filesPerLevel; j++ {
			path := filepath.Join(current, fmt.Sprintf("file-%02d.txt", j))
			if err := os.WriteFile(path, []byte("data\n"), 0o644); err != nil {
				b.Fatalf("write deep file: %v", err)
			}
		}
	}
	return current
}

func BenchmarkURootLsFlat(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("ls", true)
	registry.ConfigureRateLimits(RateLimitConfig{DefaultPerMinute: 0})

	dir, cleanup := setupURootBenchDir(b)
	defer cleanup()
	populateFlatDir(b, dir, 200)

	args := map[string]interface{}{"path": relPath(b, dir)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if result := registry.Execute("ls", args); result.Error != nil {
			b.Fatalf("ls failed: %v", result.Error)
		}
	}
}

func BenchmarkURootLsRecursiveDeep(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("ls", true)
	registry.ConfigureRateLimits(RateLimitConfig{DefaultPerMinute: 0})

	dir, cleanup := setupURootBenchDir(b)
	defer cleanup()
	populateDeepDir(b, dir, 6, 10)

	args := map[string]interface{}{
		"path":      relPath(b, dir),
		"recursive": true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if result := registry.Execute("ls", args); result.Error != nil {
			b.Fatalf("ls recursive failed: %v", result.Error)
		}
	}
}

func BenchmarkURootCatLargeFile(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("cat", true)
	registry.ConfigureRateLimits(RateLimitConfig{DefaultPerMinute: 0})

	dir, cleanup := setupURootBenchDir(b)
	defer cleanup()

	path := filepath.Join(dir, "large.txt")
	writeBenchFile(b, path, 2000, 0)

	args := map[string]interface{}{"path": relPath(b, path)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if result := registry.Execute("cat", args); result.Error != nil {
			b.Fatalf("cat failed: %v", result.Error)
		}
	}
}

func BenchmarkURootGrep(b *testing.B) {
	registry := NewRegistry()
	registry.SetAllowed("grep", true)
	registry.ConfigureRateLimits(RateLimitConfig{DefaultPerMinute: 0})

	dir, cleanup := setupURootBenchDir(b)
	defer cleanup()

	path := filepath.Join(dir, "grep.txt")
	writeBenchFile(b, path, 1500, 10)

	args := map[string]interface{}{
		"pattern": "needle",
		"path":    relPath(b, path),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if result := registry.Execute("grep", args); result.Error != nil {
			b.Fatalf("grep failed: %v", result.Error)
		}
	}
}
