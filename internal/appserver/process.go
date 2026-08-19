package appserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"promptline/internal/instance"
)

// Process owns one non-daemon app-server child and its protocol client.
type Process struct {
	cmd         *exec.Cmd
	Client      *Client
	stderr      bytes.Buffer
	stderrLimit int
	wait        chan error
	once        sync.Once
}

var sensitiveDiagnostic = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|authorization|cookie)\s*[:=]\s*[^\s]+`)

func Start(ctx context.Context, in *instance.Instance) (*Process, error) {
	startupCtx, cancel := context.WithTimeout(ctx, in.Timeouts().Startup)
	defer cancel()
	if err := Probe(startupCtx, in.CodexExecutable()); err != nil {
		return nil, err
	}
	if err := startupCtx.Err(); err != nil {
		return nil, fmt.Errorf("app-server startup: %w", err)
	}
	cmd := exec.CommandContext(ctx, in.CodexExecutable(), "app-server", "--stdio")
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
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("app-server stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start app-server: %w", err)
	}
	p := &Process{cmd: cmd, stderrLimit: in.OutputCaps().StderrBytes, wait: make(chan error, 1)}
	p.Client = New(stdin, stdout, Config{Limits: Limits{MaxFrameBytes: in.OutputCaps().StdoutBytes}})
	go p.capture(stderr)
	go func() { p.wait <- cmd.Wait(); close(p.wait) }()
	return p, nil
}
func (p *Process) capture(r io.Reader) {
	_, _ = io.Copy(&boundedWriter{b: &p.stderr, n: p.stderrLimit}, r)
}
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
