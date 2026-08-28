package appserver

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"promptline/internal/instance"
)

func TestBoundedWriterAndRedaction(t *testing.T) {
	var b bytes.Buffer
	w := boundedWriter{b: &b, n: 4}
	if _, err := w.Write([]byte("abcdef")); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != "abcd" {
		t.Fatalf("bounded output = %q", got)
	}
	p := Process{}
	p.stderr.WriteString("token=super-secret normal")
	if got := p.Stderr(); strings.Contains(got, "super-secret") || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("stderr redaction = %q", got)
	}
}

func TestStartWithPropagatesInjectedLaunchFault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'codex-cli 0.149.0'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	in, err := instance.New(instance.Config{Name: "process", StateRoot: t.TempDir(), WorkingRoot: t.TempDir(), CodexExecutable: path})
	if err != nil {
		t.Fatal(err)
	}
	fault := errors.New("launch fault")
	_, err = StartWith(context.Background(), in, func(*exec.Cmd) error { return fault })
	if !errors.Is(err, fault) {
		t.Fatalf("StartWith error = %v", err)
	}
}

func TestStartWithUsesIsolatedSkillArguments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'codex-cli 0.149.0'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	in, err := instance.New(instance.Config{Name: "isolated", StateRoot: t.TempDir(), WorkingRoot: t.TempDir(), CodexExecutable: path})
	if err != nil {
		t.Fatal(err)
	}
	fault := errors.New("stop after argv capture")
	var arguments []string
	var environment []string
	_, err = StartWith(context.Background(), in, func(cmd *exec.Cmd) error {
		arguments = append([]string(nil), cmd.Args...)
		environment = append([]string(nil), cmd.Env...)
		return fault
	})
	if !errors.Is(err, fault) {
		t.Fatalf("StartWith error = %v", err)
	}
	want := []string{"/proc/self/fd/3", "app-server", "--stdio", "-c", "skills.include_instructions=false", "-c", "skills.bundled.enabled=false"}
	if !slices.Equal(arguments, want) {
		t.Fatalf("app-server argv = %q, want %q", arguments, want)
	}
	if !slices.Contains(environment, "CODEX_HOME="+in.CodexHome()) || !slices.Contains(environment, "CODEX_CONFIG="+in.CodexConfigPath()) {
		t.Fatalf("app-server environment does not force private configuration: %q", environment)
	}
}

func TestStartWithRejectsExecutableReplacementAfterProbe(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "codex")
	fixture := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'codex-cli 0.149.0'; fi\n"
	if err := os.WriteFile(path, []byte(fixture), 0o700); err != nil {
		t.Fatal(err)
	}
	in, err := instance.New(instance.Config{Name: "replacement", StateRoot: t.TempDir(), WorkingRoot: root, CodexExecutable: path})
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(root, "replacement")
	if err := os.WriteFile(replacement, []byte(fixture), 0o700); err != nil {
		t.Fatal(err)
	}
	launched := false
	_, err = startWith(context.Background(), in, func(context.Context, Executable) (string, error) {
		if err := os.Rename(replacement, path); err != nil {
			return "", err
		}
		return "0.149.0", nil
	}, nil, func(*exec.Cmd) error {
		launched = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("StartWith error = %v, want integrity failure", err)
	}
	if launched {
		t.Fatal("replacement executable launched")
	}
}

func TestStartWithBindsLaunchToRetainedExecutableAfterRevalidation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("verified descriptor launch is Linux-specific")
	}
	root := t.TempDir()
	path := filepath.Join(root, "codex")
	marker := filepath.Join(root, "marker")
	original := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'codex-cli 0.149.0'; else echo original > \"$MARKER\"; fi\n"
	replacement := "#!/bin/sh\necho replacement > \"$MARKER\"\n"
	if err := os.WriteFile(path, []byte(original), 0o700); err != nil {
		t.Fatal(err)
	}
	in, err := instance.New(instance.Config{Name: "bound-launch", StateRoot: t.TempDir(), WorkingRoot: root, CodexExecutable: path})
	if err != nil {
		t.Fatal(err)
	}
	replacementPath := filepath.Join(root, "replacement")
	if err := os.WriteFile(replacementPath, []byte(replacement), 0o700); err != nil {
		t.Fatal(err)
	}
	p, err := startWith(context.Background(), in, ProbeExecutable, func() {
		if err := os.Rename(replacementPath, path); err != nil {
			t.Fatal(err)
		}
	}, func(cmd *exec.Cmd) error {
		cmd.Env = append(cmd.Env, "MARKER="+marker)
		return cmd.Start()
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "original\n" {
		t.Fatalf("launched marker = %q, err = %v", data, err)
	}
}

func TestProcessReportsCodexVersion(t *testing.T) {
	p := Process{codexVersion: "0.149.0"}
	if got := p.CodexVersion(); got != "0.149.0" {
		t.Fatalf("CodexVersion() = %q, want 0.149.0", got)
	}
}

func TestStart_RealChildLifecycleAndDiagnostics(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "fixture-codex")
	fixture := `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo 'codex-cli 9.8.7'
  exit 0
fi
if [ "$1" = "app-server" ] && [ "$2" = "--stdio" ]; then
  echo 'token=never-log-this' >&2
  cat >/dev/null
  exit 0
fi
exit 23
`
	if err := os.WriteFile(executable, []byte(fixture), 0o700); err != nil {
		t.Fatal(err)
	}
	in, err := instance.New(instance.Config{
		Name:            "real-child",
		StateRoot:       filepath.Join(root, "state"),
		WorkingRoot:     root,
		CodexExecutable: executable,
		Timeouts:        instance.Timeouts{Startup: time.Second, Shutdown: time.Second},
		OutputCaps:      instance.OutputCaps{StdoutBytes: 1024, StderrBytes: 64},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Start(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.CodexVersion(); got != "9.8.7" {
		t.Fatalf("CodexVersion() = %q", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if stderr := p.Stderr(); strings.Contains(stderr, "never-log-this") || !strings.Contains(stderr, "[REDACTED]") {
		t.Fatalf("stderr = %q", stderr)
	}
	select {
	case _, ok := <-p.wait:
		if ok {
			t.Fatal("process wait channel remains open")
		}
	case <-time.After(time.Second):
		t.Fatal("process was not reaped")
	}
}

func TestStart_RejectsMissingOrMalformedProbe(t *testing.T) {
	root := t.TempDir()
	for _, tt := range []struct {
		name    string
		path    string
		content string
	}{
		{name: "missing executable", path: filepath.Join(root, "missing")},
		{name: "malformed version", path: filepath.Join(root, "bad"), content: "#!/bin/sh\necho nope\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.content != "" {
				if err := os.WriteFile(tt.path, []byte(tt.content), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			in, err := instance.New(instance.Config{Name: tt.name[:3], StateRoot: filepath.Join(t.TempDir(), "state"), WorkingRoot: root, CodexExecutable: tt.path})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Start(context.Background(), in); err == nil {
				t.Fatal("Start() succeeded")
			}
		})
	}
}

func TestProcess_CloseDeadlineKillsUncooperativeChild(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "slow-codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'codex-cli 1.2.3'; else exec sleep 30; fi\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	in, err := instance.New(instance.Config{Name: "slow-child", StateRoot: filepath.Join(root, "state"), WorkingRoot: root, CodexExecutable: executable, Timeouts: instance.Timeouts{Startup: time.Second, Shutdown: time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Start(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := p.Close(ctx); err == nil {
		t.Fatal("Close() succeeded for killed child")
	}
	select {
	case _, open := <-p.wait:
		if open {
			t.Fatal("wait result was not consumed by Close")
		}
	case <-time.After(time.Second):
		t.Fatal("wait channel was not closed")
	}
}
