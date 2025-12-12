//go:build linux

package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/criyle/go-sandbox/container"
	"github.com/criyle/go-sandbox/pkg/mount"
	"github.com/criyle/go-sandbox/runner"
	"golang.org/x/sys/unix"

	"promptline/internal/config"
)

var defaultReadOnlyPaths = []string{"/bin", "/usr", "/lib", "/lib64"}

// Manager owns a single sandboxed container instance for executing commands.
type Manager struct {
	cfg      config.Sandbox
	mu       sync.Mutex
	started  bool
	disabled bool
	env      container.Environment
}

// NewManager constructs a sandbox manager from configuration.
func NewManager(cfg config.Sandbox) *Manager {
	return &Manager{
		cfg:      cfg,
		disabled: !cfg.Enabled,
	}
}

// Start builds and initializes the sandbox container. It is safe to call multiple times.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started || m.disabled {
		m.started = true
		return nil
	}

	workdir, err := m.resolveWorkdir()
	if err != nil {
		return err
	}

	roPaths := m.cfg.ReadOnlyPaths
	if len(roPaths) == 0 {
		roPaths = defaultReadOnlyPaths
	}

	mb := mount.NewBuilder()

	// Parent directory bind (read-only) to constrain traversal.
	parent := filepath.Dir(workdir)
	if parent != "/" && parent != workdir {
		if err := appendBind(mb, parent, strings.TrimPrefix(parent, "/"), true); err != nil {
			return err
		}
	}

	// Workdir bind (read-write).
	if err := appendBind(mb, workdir, "w", false); err != nil {
		return err
	}

	// Read-only host paths (binaries/libs).
	for _, p := range roPaths {
		if p == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			return fmt.Errorf("sandbox: resolve readonly path %q: %w", p, err)
		}
		target := strings.TrimPrefix(strings.TrimPrefix(p, "/"), "/")
		if target == "" {
			continue
		}
		if err := appendBind(mb, resolved, target, true); err != nil {
			return err
		}
	}

	mb.WithTmpfs("tmp", "")
	mb.WithProc()

	builder := container.Builder{
		Mounts:    mb.Mounts,
		MaskPaths: m.cfg.MaskedPaths,
		WorkDir:   "/w",
	}

	if m.cfg.NonRootUser {
		builder.CredGenerator = &credGenerator{
			uid: os.Geteuid(),
			gid: os.Getegid(),
		}
	}

	env, err := builder.Build()
	if err != nil {
		return fmt.Errorf("sandbox: build container: %w", err)
	}

	m.env = env
	m.started = true
	return nil
}

// ExecInSandbox runs a command inside the sandbox, wiring stdio to the provided streams.
func (m *Manager) ExecInSandbox(ctx context.Context, cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) (runner.Result, error) {
	if m.disabled {
		return m.execOnHost(ctx, cmd, args, stdin, stdout, stderr)
	}

	if err := m.Start(); err != nil {
		return runner.Result{}, err
	}

	m.mu.Lock()
	env := m.env
	m.mu.Unlock()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return runner.Result{}, fmt.Errorf("sandbox: stdin pipe: %w", err)
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return runner.Result{}, fmt.Errorf("sandbox: stdout pipe: %w", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return runner.Result{}, fmt.Errorf("sandbox: stderr pipe: %w", err)
	}

	wg := sync.WaitGroup{}

	if stdin != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			io.Copy(stdinW, stdin)
			stdinW.Close()
		}()
	} else {
		stdinW.Close()
	}

	if stdout != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			io.Copy(stdout, stdoutR)
		}()
	}

	if stderr != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			io.Copy(stderr, stderrR)
		}()
	}

	param := container.ExecveParam{
		Args: append([]string{cmd}, args...),
		Files: []uintptr{
			stdinR.Fd(),
			stdoutW.Fd(),
			stderrW.Fd(),
		},
	}

	result := env.Execve(ctx, param)

	stdinR.Close()
	stdoutW.Close()
	stderrW.Close()

	wg.Wait()

	return result, nil
}

// Close tears down the container environment.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.env != nil {
		err := m.env.Destroy()
		m.env = nil
		m.started = false
		return err
	}
	return nil
}

func (m *Manager) resolveWorkdir() (string, error) {
	workdir := m.cfg.Workdir
	if workdir == "" {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("sandbox: get cwd: %w", err)
		}
	}
	resolved, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		return "", fmt.Errorf("sandbox: resolve workdir %q: %w", workdir, err)
	}
	if !filepath.IsAbs(resolved) {
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return "", fmt.Errorf("sandbox: abs workdir %q: %w", resolved, err)
		}
	}
	return resolved, nil
}

func appendBind(b *mount.Builder, source, target string, readonly bool) error {
	if source == "" || target == "" {
		return nil
	}
	flags := bindFlags(readonly)
	b.Mounts = append(b.Mounts, mount.Mount{
		Source: source,
		Target: target,
		Flags:  flags,
	})
	return nil
}

func bindFlags(readonly bool) uintptr {
	flags := uintptr(unix.MS_BIND | unix.MS_NOSUID | unix.MS_PRIVATE | unix.MS_REC)
	if readonly {
		flags |= unix.MS_RDONLY
	}
	// Prevent symlink traversal on supported kernels.
	flags |= unix.MS_NOSYMFOLLOW
	return flags
}

type credGenerator struct {
	uid int
	gid int
}

func (c *credGenerator) Get() syscall.Credential {
	return syscall.Credential{
		Uid: uint32(c.uid),
		Gid: uint32(c.gid),
	}
}

func (m *Manager) execOnHost(ctx context.Context, cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) (runner.Result, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Stdin = stdin
	c.Stdout = stdout
	c.Stderr = stderr

	err := c.Run()
	if err != nil {
		return runner.Result{
			Status: runner.StatusRunnerError,
			Error:  err.Error(),
		}, err
	}

	return runner.Result{
		Status: runner.StatusNormal,
	}, nil
}
