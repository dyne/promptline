package appserver

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
	select {
	case <-p.captured:
	case <-time.After(time.Second):
		t.Fatal("stderr capture did not finish")
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
