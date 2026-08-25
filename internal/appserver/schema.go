package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`^codex-cli ([0-9]+\.[0-9]+\.[0-9]+)$`)

// Probe verifies an executable and returns its reported version before a child
// is started. It never invokes a shell or rejects a well-formed version merely
// because it differs from the reference fixture.
func Probe(ctx context.Context, executable string) (string, error) {
	if executable == "" {
		executable = "codex"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return "", fmt.Errorf("find codex executable: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
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
