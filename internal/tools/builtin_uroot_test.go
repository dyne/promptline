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
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestURootFileOperations(t *testing.T) {
	ConfigureLimits(DefaultLimits())
	t.Cleanup(func() {
		ConfigureLimits(DefaultLimits())
	})

	t.Run("cat", func(t *testing.T) {
		registry := NewRegistry()
		dir := makeTempDir(t)
		path := writeTestFile(t, dir, "sample.txt", "hello\nworld")
		rel := relPath(t, path)

		result := executeTool(t, registry, "cat", map[string]interface{}{"path": rel})
		if result.Error != nil {
			t.Fatalf("expected cat success, got %v", result.Error)
		}
		if strings.TrimSpace(result.Result) != "hello\nworld" {
			t.Fatalf("unexpected cat output: %q", result.Result)
		}
	})

	t.Run("cp and mv", func(t *testing.T) {
		registry := NewRegistry()
		dir := makeTempDir(t)
		src := writeTestFile(t, dir, "source.txt", "copy me")
		relSrc := relPath(t, src)
		dest := filepath.Join(dir, "dest.txt")
		relDest := relPath(t, dest)

		cpResult := executeTool(t, registry, "cp", map[string]interface{}{
			"sources":     []string{relSrc},
			"destination": relDest,
		})
		if cpResult.Error != nil {
			t.Fatalf("expected cp success, got %v", cpResult.Error)
		}
		assertFileContent(t, dest, "copy me")

		mvDest := filepath.Join(dir, "moved.txt")
		mvResult := executeTool(t, registry, "mv", map[string]interface{}{
			"sources":     []string{relDest},
			"destination": relPath(t, mvDest),
		})
		if mvResult.Error != nil {
			t.Fatalf("expected mv success, got %v", mvResult.Error)
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatalf("expected original destination removed after mv")
		}
		assertFileContent(t, mvDest, "copy me")
	})

	t.Run("rm", func(t *testing.T) {
		registry := NewRegistry()
		dir := makeTempDir(t)
		target := writeTestFile(t, dir, "remove.txt", "gone")

		rmResult := executeTool(t, registry, "rm", map[string]interface{}{"path": relPath(t, target)})
		if rmResult.Error != nil {
			t.Fatalf("expected rm success, got %v", rmResult.Error)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("expected file to be removed")
		}
	})

	t.Run("ln", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink tests require elevated privileges on windows")
		}
		registry := NewRegistry()
		dir := makeTempDir(t)
		target := writeTestFile(t, dir, "target.txt", "linked")
		linkPath := filepath.Join(dir, "link.txt")

		lnResult := executeTool(t, registry, "ln", map[string]interface{}{
			"target":    relPath(t, target),
			"link_path": relPath(t, linkPath),
			"symbolic":  true,
		})
		if lnResult.Error != nil {
			t.Fatalf("expected ln success, got %v", lnResult.Error)
		}
		linkTarget, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatalf("expected symlink, got %v", err)
		}
		if filepath.Base(linkTarget) != "target.txt" {
			t.Fatalf("unexpected link target: %q", linkTarget)
		}
	})

	t.Run("touch and truncate", func(t *testing.T) {
		registry := NewRegistry()
		dir := makeTempDir(t)
		path := filepath.Join(dir, "empty.txt")

		touchResult := executeTool(t, registry, "touch", map[string]interface{}{"path": relPath(t, path)})
		if touchResult.Error != nil {
			t.Fatalf("expected touch success, got %v", touchResult.Error)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected file created, got %v", err)
		}
		if info.Size() != 0 {
			t.Fatalf("expected empty file, got size %d", info.Size())
		}

		truncateResult := executeTool(t, registry, "truncate", map[string]interface{}{
			"path": relPath(t, path),
			"size": 5,
		})
		if truncateResult.Error != nil {
			t.Fatalf("expected truncate success, got %v", truncateResult.Error)
		}
		info, err = os.Stat(path)
		if err != nil {
			t.Fatalf("expected file exists, got %v", err)
		}
		if info.Size() != 5 {
			t.Fatalf("expected size 5, got %d", info.Size())
		}
	})

	t.Run("readlink and realpath", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink tests require elevated privileges on windows")
		}
		registry := NewRegistry()
		dir := makeTempDir(t)
		target := writeTestFile(t, dir, "real.txt", "real")
		linkPath := filepath.Join(dir, "real-link.txt")
		if err := os.Symlink("real.txt", linkPath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		readlinkResult := executeTool(t, registry, "readlink", map[string]interface{}{
			"path":   relPath(t, linkPath),
			"follow": false,
		})
		if readlinkResult.Error != nil {
			t.Fatalf("expected readlink success, got %v", readlinkResult.Error)
		}
		if strings.TrimSpace(readlinkResult.Result) != "real.txt" {
			t.Fatalf("unexpected readlink output: %q", readlinkResult.Result)
		}

		realpathResult := executeTool(t, registry, "realpath", map[string]interface{}{
			"path": relPath(t, linkPath),
		})
		if realpathResult.Error != nil {
			t.Fatalf("expected realpath success, got %v", realpathResult.Error)
		}
		absTarget, err := filepath.Abs(target)
		if err != nil {
			t.Fatalf("failed to resolve abs target: %v", err)
		}
		expected, err := filepath.EvalSymlinks(absTarget)
		if err != nil {
			t.Fatalf("failed to eval symlink: %v", err)
		}
		if strings.TrimSpace(realpathResult.Result) != expected {
			t.Fatalf("unexpected realpath output: %q", realpathResult.Result)
		}
	})
}

func TestURootFileOperationsValidation(t *testing.T) {
	registry := NewRegistry()

	result := executeTool(t, registry, "cat", map[string]interface{}{"path": "/etc/passwd"})
	if result.Error == nil {
		t.Fatalf("expected error for blocked path")
	}
}

func TestURootFileOperationsSizeLimit(t *testing.T) {
	ConfigureLimits(Limits{
		MaxFileSizeBytes:    4,
		MaxDirectoryDepth:   defaultMaxDirectoryDepth,
		MaxDirectoryEntries: defaultMaxDirectoryEntries,
	})
	t.Cleanup(func() {
		ConfigureLimits(DefaultLimits())
	})

	registry := NewRegistry()
	dir := makeTempDir(t)
	path := writeTestFile(t, dir, "big.txt", "12345")

	result := executeTool(t, registry, "cat", map[string]interface{}{"path": relPath(t, path)})
	if result.Error == nil {
		t.Fatalf("expected size limit error")
	}
}

func TestURootFileOperationsRateLimit(t *testing.T) {
	registry := NewRegistry()
	registry.ConfigureRateLimits(RateLimitConfig{
		DefaultPerMinute: 0,
		PerTool: map[string]int{
			"cat": 1,
		},
	})

	dir := makeTempDir(t)
	path := writeTestFile(t, dir, "rate.txt", "rate")
	args := map[string]interface{}{"path": relPath(t, path)}

	first := executeTool(t, registry, "cat", args)
	if first.Error != nil {
		t.Fatalf("expected cat success, got %v", first.Error)
	}

	second := executeTool(t, registry, "cat", args)
	if second.Error == nil {
		t.Fatalf("expected rate limit error")
	}
	if !errors.Is(second.Error, ErrToolRateLimited) {
		t.Fatalf("expected ErrToolRateLimited, got %v", second.Error)
	}
}

func TestURootFileOperationsTimeout(t *testing.T) {
	registry := NewRegistry()
	registry.ConfigureTimeouts(TimeoutConfig{
		PerTool: map[string]time.Duration{
			"cat": time.Nanosecond,
		},
	})

	dir := makeTempDir(t)
	path := writeTestFile(t, dir, "timeout.txt", "timeout")
	result := executeTool(t, registry, "cat", map[string]interface{}{"path": relPath(t, path)})
	if result.Error == nil {
		t.Fatalf("expected timeout error")
	}
	if !errors.Is(result.Error, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", result.Error)
	}
}

func TestURootDirectoryOperations(t *testing.T) {
	registry := NewRegistry()

	t.Run("mkdir", func(t *testing.T) {
		dir := makeTempDir(t)
		parent := filepath.Join(dir, "parent")
		if err := os.MkdirAll(parent, 0o755); err != nil {
			t.Fatalf("failed to create parent dir: %v", err)
		}
		target := filepath.Join(parent, "child")
		result := executeTool(t, registry, "mkdir", map[string]interface{}{
			"path":    relPath(t, target),
			"parents": true,
		})
		if result.Error != nil {
			t.Fatalf("expected mkdir success, got %v", result.Error)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("expected directory created, got %v", err)
		}
	})

	t.Run("pwd", func(t *testing.T) {
		result := executeTool(t, registry, "pwd", map[string]interface{}{})
		if result.Error != nil {
			t.Fatalf("expected pwd success, got %v", result.Error)
		}
		workdir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		if strings.TrimSpace(result.Result) != workdir {
			t.Fatalf("unexpected pwd output: %q", result.Result)
		}
	})

	t.Run("dirname and basename", func(t *testing.T) {
		path := filepath.Join("a", "b", "c.txt")
		dirResult := executeTool(t, registry, "dirname", map[string]interface{}{"path": path})
		if dirResult.Error != nil {
			t.Fatalf("expected dirname success, got %v", dirResult.Error)
		}
		if strings.TrimSpace(dirResult.Result) != filepath.Dir(path) {
			t.Fatalf("unexpected dirname output: %q", dirResult.Result)
		}

		baseResult := executeTool(t, registry, "basename", map[string]interface{}{"path": path})
		if baseResult.Error != nil {
			t.Fatalf("expected basename success, got %v", baseResult.Error)
		}
		if strings.TrimSpace(baseResult.Result) != filepath.Base(path) {
			t.Fatalf("unexpected basename output: %q", baseResult.Result)
		}
	})
}

func TestURootTextProcessing(t *testing.T) {
	registry := NewRegistry()
	dir := makeTempDir(t)

	textPath := writeTestFile(t, dir, "text.txt", "alpha\nbeta\nbeta\ngamma")
	unsortedPath := writeTestFile(t, dir, "unsorted.txt", "c\nb\na\n")
	otherPath := writeTestFile(t, dir, "other.txt", "beta\ndelta\n")
	stringPath := filepath.Join(dir, "binary.bin")
	if err := os.WriteFile(stringPath, []byte{0x00, 'h', 'e', 'l', 'l', 'o', 0x00, 'w', 'o', 'r', 'l', 'd', 0x00}, 0o644); err != nil {
		t.Fatalf("failed to write binary file: %v", err)
	}

	t.Run("grep", func(t *testing.T) {
		result := executeTool(t, registry, "grep", map[string]interface{}{
			"pattern": "beta",
			"path":    relPath(t, textPath),
		})
		if result.Error != nil {
			t.Fatalf("expected grep success, got %v", result.Error)
		}
		if strings.TrimSpace(result.Result) != "beta\nbeta" {
			t.Fatalf("unexpected grep output: %q", result.Result)
		}
	})

	t.Run("head and tail", func(t *testing.T) {
		headResult := executeTool(t, registry, "head", map[string]interface{}{
			"path":  relPath(t, textPath),
			"lines": 2,
		})
		if headResult.Error != nil {
			t.Fatalf("expected head success, got %v", headResult.Error)
		}
		if strings.TrimSpace(headResult.Result) != "alpha\nbeta" {
			t.Fatalf("unexpected head output: %q", headResult.Result)
		}

		tailResult := executeTool(t, registry, "tail", map[string]interface{}{
			"path":  relPath(t, textPath),
			"lines": 2,
		})
		if tailResult.Error != nil {
			t.Fatalf("expected tail success, got %v", tailResult.Error)
		}
		if strings.TrimSpace(tailResult.Result) != "beta\ngamma" {
			t.Fatalf("unexpected tail output: %q", tailResult.Result)
		}
	})

	t.Run("sort and uniq", func(t *testing.T) {
		sortResult := executeTool(t, registry, "sort", map[string]interface{}{
			"path": relPath(t, unsortedPath),
		})
		if sortResult.Error != nil {
			t.Fatalf("expected sort success, got %v", sortResult.Error)
		}
		if strings.TrimSpace(sortResult.Result) != "a\nb\nc" {
			t.Fatalf("unexpected sort output: %q", sortResult.Result)
		}

		uniqResult := executeTool(t, registry, "uniq", map[string]interface{}{
			"path": relPath(t, textPath),
		})
		if uniqResult.Error != nil {
			t.Fatalf("expected uniq success, got %v", uniqResult.Error)
		}
		if strings.TrimSpace(uniqResult.Result) != "alpha\nbeta\ngamma" {
			t.Fatalf("unexpected uniq output: %q", uniqResult.Result)
		}
	})

	t.Run("wc and tr", func(t *testing.T) {
		wcResult := executeTool(t, registry, "wc", map[string]interface{}{
			"path": relPath(t, textPath),
		})
		if wcResult.Error != nil {
			t.Fatalf("expected wc success, got %v", wcResult.Error)
		}
		if strings.TrimSpace(wcResult.Result) == "" {
			t.Fatalf("unexpected wc output: %q", wcResult.Result)
		}

		trResult := executeTool(t, registry, "tr", map[string]interface{}{
			"from":  "a",
			"to":    "o",
			"input": "alpha",
		})
		if trResult.Error != nil {
			t.Fatalf("expected tr success, got %v", trResult.Error)
		}
		if strings.TrimSpace(trResult.Result) != "olpho" {
			t.Fatalf("unexpected tr output: %q", trResult.Result)
		}
	})

	t.Run("tee", func(t *testing.T) {
		teeTarget := filepath.Join(dir, "tee.txt")
		result := executeTool(t, registry, "tee", map[string]interface{}{
			"content": "tee content",
			"path":    relPath(t, teeTarget),
		})
		if result.Error != nil {
			t.Fatalf("expected tee success, got %v", result.Error)
		}
		assertFileContent(t, teeTarget, "tee content")
		if strings.TrimSpace(result.Result) != "tee content" {
			t.Fatalf("unexpected tee output: %q", result.Result)
		}
	})

	t.Run("comm", func(t *testing.T) {
		commResult := executeTool(t, registry, "comm", map[string]interface{}{
			"path1": relPath(t, textPath),
			"path2": relPath(t, otherPath),
		})
		if commResult.Error != nil {
			t.Fatalf("expected comm success, got %v", commResult.Error)
		}
		if strings.TrimSpace(commResult.Result) == "" {
			t.Fatalf("unexpected comm output: %q", commResult.Result)
		}
	})

	t.Run("strings", func(t *testing.T) {
		result := executeTool(t, registry, "strings", map[string]interface{}{
			"path":       relPath(t, stringPath),
			"min_length": 5,
		})
		if result.Error != nil {
			t.Fatalf("expected strings success, got %v", result.Error)
		}
		if !strings.Contains(result.Result, "hello") || !strings.Contains(result.Result, "world") {
			t.Fatalf("unexpected strings output: %q", result.Result)
		}
	})

	t.Run("more", func(t *testing.T) {
		result := executeTool(t, registry, "more", map[string]interface{}{
			"path":  relPath(t, textPath),
			"lines": 2,
		})
		if result.Error != nil {
			t.Fatalf("expected more success, got %v", result.Error)
		}
		if strings.TrimSpace(result.Result) != "alpha\nbeta" {
			t.Fatalf("unexpected more output: %q", result.Result)
		}
	})
}

func TestURootFileViewingAnalysis(t *testing.T) {
	registry := NewRegistry()
	dir := makeTempDir(t)

	filePath := writeTestFile(t, dir, "data.txt", "hash me")
	otherPath := writeTestFile(t, dir, "other.txt", "hash me")
	diffPath := writeTestFile(t, dir, "diff.txt", "hash you")
	base64Path := writeTestFile(t, dir, "encoded.txt", base64.StdEncoding.EncodeToString([]byte("data")))

	t.Run("hexdump", func(t *testing.T) {
		result := executeTool(t, registry, "hexdump", map[string]interface{}{
			"path":      relPath(t, filePath),
			"max_bytes": 16,
		})
		if result.Error != nil {
			t.Fatalf("expected hexdump success, got %v", result.Error)
		}
		if !strings.Contains(result.Result, "68 61 73 68") {
			t.Fatalf("unexpected hexdump output: %q", result.Result)
		}
	})

	t.Run("cmp", func(t *testing.T) {
		result := executeTool(t, registry, "cmp", map[string]interface{}{
			"path1": relPath(t, filePath),
			"path2": relPath(t, otherPath),
		})
		if result.Error != nil {
			t.Fatalf("expected cmp success, got %v", result.Error)
		}
		if strings.TrimSpace(result.Result) != "Files are identical" {
			t.Fatalf("unexpected cmp output: %q", result.Result)
		}

		diffResult := executeTool(t, registry, "cmp", map[string]interface{}{
			"path1": relPath(t, filePath),
			"path2": relPath(t, diffPath),
		})
		if diffResult.Error != nil {
			t.Fatalf("expected cmp success, got %v", diffResult.Error)
		}
		if !strings.Contains(diffResult.Result, "Files differ") {
			t.Fatalf("unexpected cmp diff output: %q", diffResult.Result)
		}
	})

	t.Run("md5sum", func(t *testing.T) {
		sum := md5.Sum([]byte("hash me"))
		expected := hex.EncodeToString(sum[:])
		result := executeTool(t, registry, "md5sum", map[string]interface{}{
			"path": relPath(t, filePath),
		})
		if result.Error != nil {
			t.Fatalf("expected md5sum success, got %v", result.Error)
		}
		if !strings.HasPrefix(strings.TrimSpace(result.Result), expected) {
			t.Fatalf("unexpected md5sum output: %q", result.Result)
		}
	})

	t.Run("shasum", func(t *testing.T) {
		sum := sha1.Sum([]byte("hash me"))
		expected := hex.EncodeToString(sum[:])
		result := executeTool(t, registry, "shasum", map[string]interface{}{
			"path": relPath(t, filePath),
		})
		if result.Error != nil {
			t.Fatalf("expected shasum success, got %v", result.Error)
		}
		if !strings.HasPrefix(strings.TrimSpace(result.Result), expected) {
			t.Fatalf("unexpected shasum output: %q", result.Result)
		}
	})

	t.Run("base64", func(t *testing.T) {
		encoded := executeTool(t, registry, "base64", map[string]interface{}{
			"path": relPath(t, filePath),
		})
		if encoded.Error != nil {
			t.Fatalf("expected base64 encode success, got %v", encoded.Error)
		}
		expected := base64.StdEncoding.EncodeToString([]byte("hash me"))
		if strings.TrimSpace(encoded.Result) != expected {
			t.Fatalf("unexpected base64 encode output: %q", encoded.Result)
		}

		decoded := executeTool(t, registry, "base64", map[string]interface{}{
			"path":   relPath(t, base64Path),
			"decode": true,
		})
		if decoded.Error != nil {
			t.Fatalf("expected base64 decode success, got %v", decoded.Error)
		}
		if strings.TrimSpace(decoded.Result) != "data" {
			t.Fatalf("unexpected base64 decode output: %q", decoded.Result)
		}
	})
}

func TestURootSystemInformation(t *testing.T) {
	registry := NewRegistry()
	dir := makeTempDir(t)

	t.Run("uname and hostname", func(t *testing.T) {
		unameResult := executeTool(t, registry, "uname", map[string]interface{}{})
		if unameResult.Error != nil {
			t.Fatalf("expected uname success, got %v", unameResult.Error)
		}
		if strings.TrimSpace(unameResult.Result) == "" {
			t.Fatalf("unexpected uname output: %q", unameResult.Result)
		}

		hostnameResult := executeTool(t, registry, "hostname", map[string]interface{}{})
		if hostnameResult.Error != nil {
			t.Fatalf("expected hostname success, got %v", hostnameResult.Error)
		}
		if strings.TrimSpace(hostnameResult.Result) == "" {
			t.Fatalf("unexpected hostname output: %q", hostnameResult.Result)
		}
	})

	t.Run("uptime and free", func(t *testing.T) {
		uptimeResult := executeTool(t, registry, "uptime", map[string]interface{}{})
		if runtime.GOOS == "windows" {
			if uptimeResult.Error == nil {
				t.Fatalf("expected uptime error on windows")
			}
		} else if uptimeResult.Error != nil {
			t.Fatalf("expected uptime success, got %v", uptimeResult.Error)
		}
		if runtime.GOOS != "windows" && !strings.Contains(uptimeResult.Result, "up") {
			t.Fatalf("unexpected uptime output: %q", uptimeResult.Result)
		}

		freeResult := executeTool(t, registry, "free", map[string]interface{}{})
		if runtime.GOOS == "windows" {
			if freeResult.Error == nil {
				t.Fatalf("expected free error on windows")
			}
		} else if freeResult.Error != nil {
			t.Fatalf("expected free success, got %v", freeResult.Error)
		}
		if runtime.GOOS != "windows" && !strings.Contains(freeResult.Result, "Mem") {
			t.Fatalf("unexpected free output: %q", freeResult.Result)
		}
	})

	t.Run("df and du", func(t *testing.T) {
		dfResult := executeTool(t, registry, "df", map[string]interface{}{
			"path": relPath(t, dir),
		})
		if dfResult.Error != nil {
			t.Fatalf("expected df success, got %v", dfResult.Error)
		}
		if !strings.Contains(dfResult.Result, "Path:") {
			t.Fatalf("unexpected df output: %q", dfResult.Result)
		}

		duResult := executeTool(t, registry, "du", map[string]interface{}{
			"path": relPath(t, dir),
		})
		if duResult.Error != nil {
			t.Fatalf("expected du success, got %v", duResult.Error)
		}
		if !strings.Contains(duResult.Result, "Total:") {
			t.Fatalf("unexpected du output: %q", duResult.Result)
		}
	})

	t.Run("ps and pidof", func(t *testing.T) {
		psResult := executeTool(t, registry, "ps", map[string]interface{}{
			"limit": 5,
		})
		if psResult.Error != nil {
			t.Fatalf("expected ps success, got %v", psResult.Error)
		}
		if !strings.Contains(psResult.Result, "PID COMMAND") {
			t.Fatalf("unexpected ps output: %q", psResult.Result)
		}

		procName := filepath.Base(os.Args[0])
		pidofResult := executeTool(t, registry, "pidof", map[string]interface{}{
			"name": procName,
		})
		if pidofResult.Error != nil {
			t.Fatalf("expected pidof success, got %v", pidofResult.Error)
		}
		if !strings.Contains(pidofResult.Result, strconv.Itoa(os.Getpid())) {
			t.Fatalf("unexpected pidof output: %q", pidofResult.Result)
		}
	})

	t.Run("id", func(t *testing.T) {
		idResult := executeTool(t, registry, "id", map[string]interface{}{})
		if idResult.Error != nil {
			t.Fatalf("expected id success, got %v", idResult.Error)
		}
		current, err := user.Current()
		if err != nil {
			t.Fatalf("failed to get current user: %v", err)
		}
		if !strings.Contains(idResult.Result, "uid="+current.Uid) {
			t.Fatalf("unexpected id output: %q", idResult.Result)
		}
	})
}

func TestURootMiscellaneousSafe(t *testing.T) {
	registry := NewRegistry()
	dir := makeTempDir(t)

	t.Run("echo and seq", func(t *testing.T) {
		echoResult := executeTool(t, registry, "echo", map[string]interface{}{
			"text": "hello world",
		})
		if echoResult.Error != nil {
			t.Fatalf("expected echo success, got %v", echoResult.Error)
		}
		if strings.TrimSpace(echoResult.Result) != "hello world" {
			t.Fatalf("unexpected echo output: %q", echoResult.Result)
		}

		seqResult := executeTool(t, registry, "seq", map[string]interface{}{
			"start": 1,
			"end":   3,
		})
		if seqResult.Error != nil {
			t.Fatalf("expected seq success, got %v", seqResult.Error)
		}
		if strings.TrimSpace(seqResult.Result) != "1\n2\n3" {
			t.Fatalf("unexpected seq output: %q", seqResult.Result)
		}
	})

	t.Run("printenv and tty", func(t *testing.T) {
		t.Setenv("PROMPTLINE_TEST_ENV", "value")
		printenvResult := executeTool(t, registry, "printenv", map[string]interface{}{
			"name": "PROMPTLINE_TEST_ENV",
		})
		if printenvResult.Error != nil {
			t.Fatalf("expected printenv success, got %v", printenvResult.Error)
		}
		if strings.TrimSpace(printenvResult.Result) != "value" {
			t.Fatalf("unexpected printenv output: %q", printenvResult.Result)
		}

		ttyResult := executeTool(t, registry, "tty", map[string]interface{}{})
		if ttyResult.Error != nil {
			t.Fatalf("expected tty success, got %v", ttyResult.Error)
		}
		if strings.TrimSpace(ttyResult.Result) == "" {
			t.Fatalf("unexpected tty output: %q", ttyResult.Result)
		}
	})

	t.Run("which", func(t *testing.T) {
		result := executeTool(t, registry, "which", map[string]interface{}{
			"name": "ls",
		})
		if result.Error != nil {
			t.Fatalf("expected which success, got %v", result.Error)
		}
		if strings.TrimSpace(result.Result) == "" {
			t.Fatalf("unexpected which output: %q", result.Result)
		}
	})

	t.Run("mkfifo and mktemp", func(t *testing.T) {
		fifoPath := filepath.Join(dir, "pipe")
		mkfifoResult := executeTool(t, registry, "mkfifo", map[string]interface{}{
			"path": relPath(t, fifoPath),
		})
		if runtime.GOOS == "windows" {
			if mkfifoResult.Error == nil {
				t.Fatalf("expected mkfifo error on windows")
			}
		} else if mkfifoResult.Error != nil {
			t.Fatalf("expected mkfifo success, got %v", mkfifoResult.Error)
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(fifoPath)
			if err != nil {
				t.Fatalf("failed to stat fifo: %v", err)
			}
			if info.Mode()&os.ModeNamedPipe == 0 {
				t.Fatalf("expected named pipe, got mode %v", info.Mode())
			}
		}

		mktempResult := executeTool(t, registry, "mktemp", map[string]interface{}{})
		if mktempResult.Error != nil {
			t.Fatalf("expected mktemp success, got %v", mktempResult.Error)
		}
		if _, err := os.Stat(strings.TrimSpace(mktempResult.Result)); err != nil {
			t.Fatalf("expected mktemp file, got %v", err)
		}
	})

	t.Run("find and chmod", func(t *testing.T) {
		target := writeTestFile(t, dir, "find.txt", "find me")
		findResult := executeTool(t, registry, "find", map[string]interface{}{
			"path": relPath(t, dir),
			"name": "*.txt",
		})
		if findResult.Error != nil {
			t.Fatalf("expected find success, got %v", findResult.Error)
		}
		if !strings.Contains(findResult.Result, target) {
			t.Fatalf("unexpected find output: %q", findResult.Result)
		}

		chmodResult := executeTool(t, registry, "chmod", map[string]interface{}{
			"path": relPath(t, target),
			"mode": "600",
		})
		if chmodResult.Error != nil {
			t.Fatalf("expected chmod success, got %v", chmodResult.Error)
		}
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("unexpected chmod mode: %v", info.Mode().Perm())
		}
	})

	t.Run("date", func(t *testing.T) {
		result := executeTool(t, registry, "date", map[string]interface{}{
			"format": "unix",
		})
		if result.Error != nil {
			t.Fatalf("expected date success, got %v", result.Error)
		}
		if _, err := strconv.ParseInt(strings.TrimSpace(result.Result), 10, 64); err != nil {
			t.Fatalf("unexpected date output: %q", result.Result)
		}
	})
}

func makeTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "uroot-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return path
}

func relPath(t testing.TB, abs string) string {
	t.Helper()
	rel, err := filepath.Rel(".", abs)
	if err != nil {
		t.Fatalf("failed to build relative path: %v", err)
	}
	return rel
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != expected {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func executeTool(t *testing.T, registry *Registry, name string, args map[string]interface{}) *ToolResult {
	t.Helper()
	return registry.ExecuteWithOptions(name, args, ExecuteOptions{Force: true})
}
