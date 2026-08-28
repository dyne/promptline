package appserver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"promptline/internal/instance"
)

// Process owns one non-daemon app-server child and its protocol client.
type Process struct {
	cmd          *exec.Cmd
	Client       *Client
	codexVersion string
	stderr       bytes.Buffer
	stderrLimit  int
	wait         chan error
	once         sync.Once
}

var sensitiveDiagnostic = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|authorization|cookie)\s*[:=]\s*[^\s]+`)

func Start(ctx context.Context, in *instance.Instance) (*Process, error) {
	return StartWith(ctx, in, func(cmd *exec.Cmd) error { return cmd.Start() })
}

// StartWith is the injectable process-launch boundary; Start always supplies
// the real exec.Cmd.Start production dependency.
func StartWith(ctx context.Context, in *instance.Instance, launch func(*exec.Cmd) error) (*Process, error) {
	return startWith(ctx, in, ProbeExecutable, nil, launch)
}

func startWith(ctx context.Context, in *instance.Instance, probe func(context.Context, Executable) (string, error), beforeLaunch func(), launch func(*exec.Cmd) error) (*Process, error) {
	if launch == nil {
		launch = func(cmd *exec.Cmd) error { return cmd.Start() }
	}
	startupCtx, cancel := context.WithTimeout(ctx, in.Timeouts().Startup)
	defer cancel()
	executable, err := ResolveExecutable(in.CodexExecutable())
	if err != nil {
		return nil, err
	}
	defer executable.Close()
	codexVersion, err := probe(startupCtx, executable)
	if err != nil {
		return nil, err
	}
	if err := startupCtx.Err(); err != nil {
		return nil, fmt.Errorf("app-server startup: %w", err)
	}
	if err := executable.Revalidate(); err != nil {
		return nil, fmt.Errorf("app-server startup integrity: %w", err)
	}
	if beforeLaunch != nil {
		beforeLaunch()
	}
	cmd, err := boundCommand(ctx, executable,
		"app-server",
		"--stdio",
		"-c",
		"skills.include_instructions=false",
		"-c",
		"skills.bundled.enabled=false",
	)
	if err != nil {
		return nil, fmt.Errorf("app-server startup integrity: %w", err)
	}
	cmd.Dir = in.WorkingDirectory()
	cmd.Env = in.EnvironmentForChild(os.Environ(), nil, nil)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("app-server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("app-server stdout: %w", err)
	}
	p := &Process{
		cmd:          cmd,
		codexVersion: codexVersion,
		stderrLimit:  in.OutputCaps().StderrBytes,
		wait:         make(chan error, 1),
	}
	cmd.Stderr = &boundedWriter{b: &p.stderr, n: p.stderrLimit}
	if err := launch(cmd); err != nil {
		return nil, fmt.Errorf("start app-server: %w", err)
	}
	p.Client = New(stdin, stdout, Config{Limits: Limits{MaxFrameBytes: in.OutputCaps().StdoutBytes}})
	go func() { p.wait <- cmd.Wait(); close(p.wait) }()
	return p, nil
}
func (p *Process) CodexVersion() string { return p.codexVersion }
func (p *Process) Stderr() string {
	return sensitiveDiagnostic.ReplaceAllString(p.stderr.String(), "$1=[REDACTED]")
}
func (p *Process) Close(ctx context.Context) error {
	var out error
	p.once.Do(func() {
		_ = p.Client.Close()
		select {
		case err := <-p.wait:
			out = err
		case <-ctx.Done():
			_ = p.cmd.Process.Kill()
			select {
			case err := <-p.wait:
				out = err
			case <-time.After(time.Second):
				out = ctx.Err()
			}
		}
	})
	return out
}

type boundedWriter struct {
	b *bytes.Buffer
	n int
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if w.b.Len() < w.n {
		left := w.n - w.b.Len()
		if left > len(p) {
			left = len(p)
		}
		_, _ = w.b.Write(p[:left])
	}
	return len(p), nil
}
