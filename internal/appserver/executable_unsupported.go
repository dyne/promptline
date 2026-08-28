//go:build !linux

package appserver

import (
	"context"
	"errors"
	"os/exec"
)

// Fail closed where this build has no verified-object exec primitive.
func boundCommand(context.Context, Executable, ...string) (*exec.Cmd, error) {
	return nil, errors.New("verified Codex executable launch is unsupported on this platform")
}
