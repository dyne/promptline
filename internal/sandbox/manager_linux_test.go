//go:build linux

package sandbox

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/criyle/go-sandbox/runner"
	"promptline/internal/config"
)

func TestManagerExecInSandboxConfinesWorkdir(t *testing.T) {
	workdir := filepath.Join("testdata", "workdir")

	mgr := NewManager(config.Sandbox{
		Enabled: true,
		Workdir: workdir,
	})
	if err := mgr.Start(); err != nil {
		t.Skipf("sandbox start unavailable in test environment: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })

	var out bytes.Buffer
	var errBuf bytes.Buffer
	result, err := mgr.ExecInSandbox(context.Background(), "sh", []string{"-c", "pwd && cat /w/hello.txt"}, nil, &out, &errBuf)
	if err != nil {
		t.Fatalf("exec failed: %v (stderr=%s)", err, errBuf.String())
	}
	if result.ExitStatus != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%s)", result.ExitStatus, errBuf.String())
	}

	output := out.String()
	if output == "" {
		t.Fatalf("expected output, got empty")
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", errBuf.String())
	}
	if !bytes.Contains([]byte(output), []byte("/w")) {
		t.Fatalf("expected workdir to be /w in container, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("hello")) {
		t.Fatalf("expected to read file inside workdir, got: %s", output)
	}
}

func TestManagerBlocksOutsideWorkdir(t *testing.T) {
	workdir := filepath.Join("testdata", "workdir")
	outside := filepath.Join("testdata", "ro", "secret.txt")

	mgr := NewManager(config.Sandbox{
		Enabled: true,
		Workdir: workdir,
	})
	if err := mgr.Start(); err != nil {
		t.Skipf("sandbox start unavailable in test environment: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })

	var out bytes.Buffer
	var errBuf bytes.Buffer
	_, err := mgr.ExecInSandbox(context.Background(), "sh", []string{"-c", "cat " + outside}, nil, &out, &errBuf)
	if err == nil {
		t.Fatalf("expected error when accessing file outside workdir; stdout=%s stderr=%s", out.String(), errBuf.String())
	}
}

func TestManagerDisabledFallback(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		// Used for potential future helper; nothing here now.
		t.Skip("helper process")
	}

	var out bytes.Buffer
	var errBuf bytes.Buffer

	mgr := NewManager(config.Sandbox{
		Enabled: false,
	})
	result, err := mgr.ExecInSandbox(context.Background(), "sh", []string{"-c", "echo ok"}, nil, &out, &errBuf)
	if err != nil {
		t.Fatalf("expected host exec fallback, got error: %v", err)
	}
	if result.Status != runner.StatusNormal {
		t.Fatalf("expected normal status, got %v", result.Status)
	}
	if out.String() == "" {
		t.Fatalf("expected stdout content")
	}
}
