//go:build linux

package appserver

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// boundCommand executes the inherited descriptor in the child namespace. The
// descriptor is retained from ResolveExecutable, so replacing its original
// pathname after validation cannot change the program execve opens.
func boundCommand(ctx context.Context, executable Executable, args ...string) (*exec.Cmd, error) {
	if executable.file == nil {
		return nil, errors.New("codex executable has no retained file descriptor")
	}
	const descriptorPath = "/proc/self/fd/3"
	cmd := exec.CommandContext(ctx, descriptorPath, args...)
	cmd.ExtraFiles = []*os.File{executable.file}
	return cmd, nil
}
