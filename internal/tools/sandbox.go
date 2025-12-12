package tools

import (
	"context"
	"io"

	"github.com/criyle/go-sandbox/runner"
)

// SandboxRunner runs commands inside an isolated sandbox.
type SandboxRunner interface {
	ExecInSandbox(ctx context.Context, cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) (runner.Result, error)
}

var sandboxRunner SandboxRunner
var sandboxWorkdir string

// SetSandboxRunner installs the sandbox runner used by execute_shell_command.
func SetSandboxRunner(r SandboxRunner, workdir string) {
	sandboxRunner = r
	sandboxWorkdir = workdir
}
