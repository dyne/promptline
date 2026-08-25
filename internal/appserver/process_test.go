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
