package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`^codex-cli ([0-9]+\.[0-9]+\.[0-9]+)$`)

// Executable is a verified Codex program. Its identity is captured before the
// version probe and rechecked immediately before the child is launched.
type Executable struct {
	path string
	info os.FileInfo
	file *os.File
}

func (e Executable) Path() string { return e.path }

// ResolveExecutable resolves a configured command once. The result is a
// regular executable file at an absolute, symlink-free path.
func ResolveExecutable(configured string) (Executable, error) {
	if configured == "" {
		configured = "codex"
	}
	path, err := exec.LookPath(configured)
	if err != nil {
		return Executable{}, fmt.Errorf("find codex executable: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return Executable{}, fmt.Errorf("make codex executable absolute: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return Executable{}, fmt.Errorf("resolve codex executable: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return Executable{}, fmt.Errorf("open codex executable: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return Executable{}, fmt.Errorf("stat codex executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return Executable{}, errors.New("codex executable is not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		_ = file.Close()
		return Executable{}, errors.New("codex executable is not executable")
	}
	return Executable{path: path, info: info, file: file}, nil
}

func (e Executable) Close() error {
	if e.file == nil {
		return nil
	}
	return e.file.Close()
}

// Revalidate refuses a replacement after probing. os.SameFile supplies the
// platform-specific file identity comparison, including Windows file IDs.
func (e Executable) Revalidate() error {
	if e.file == nil {
		return errors.New("codex executable has no retained file descriptor")
	}
	info, err := e.file.Stat()
	if err != nil {
		return fmt.Errorf("stat retained codex executable before launch: %w", err)
	}
	if !info.Mode().IsRegular() || !os.SameFile(e.info, info) {
		return errors.New("codex executable identity changed before launch")
	}
	pathInfo, err := os.Stat(e.path)
	if err != nil {
		return fmt.Errorf("stat codex executable path before launch: %w", err)
	}
	if !pathInfo.Mode().IsRegular() || !os.SameFile(e.info, pathInfo) {
		return errors.New("codex executable identity changed before launch")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return errors.New("codex executable is no longer executable")
	}
	return nil
}

// Probe verifies an executable and returns its reported version before a child
// is started. It never invokes a shell or rejects a well-formed version merely
// because it differs from the reference fixture.
func Probe(ctx context.Context, executable string) (string, error) {
	resolved, err := ResolveExecutable(executable)
	if err != nil {
		return "", err
	}
	return ProbeExecutable(ctx, resolved)
}

// ProbeExecutable verifies the exact executable object that will later launch.
func ProbeExecutable(ctx context.Context, executable Executable) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd, err := boundCommand(ctx, executable, "--version")
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("read codex version: %w", err)
	}
	return parseCodexVersion(string(out))
}

func parseCodexVersion(output string) (string, error) {
	trimmed := strings.TrimSpace(output)
	m := versionPattern.FindStringSubmatch(trimmed)
	if m == nil {
		return "", fmt.Errorf("%w: malformed codex version %q", ErrProtocol, trimmed)
	}
	return m[1], nil
}

// ValidateStableFixture protects the intentionally small stable protocol contract.
func ValidateStableFixture(data []byte) error {
	var f struct {
		CLIVersion   string `json:"cliVersion"`
		Transport    string `json:"transport"`
		Initialize   string `json:"initialize"`
		Experimental bool   `json:"experimentalApi"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&f); err != nil {
		return err
	}
	if f.CLIVersion != TestedCLIVersion || f.Transport != "stdio-jsonl" || f.Initialize != "initialize" || f.Experimental {
		return errors.New("invalid stable app-server fixture")
	}
	return nil
}
