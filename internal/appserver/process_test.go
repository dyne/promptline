package appserver

import (
	"bytes"
	"strings"
	"testing"
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

func TestProcessReportsCodexVersion(t *testing.T) {
	p := Process{codexVersion: "0.149.0"}
	if got := p.CodexVersion(); got != "0.149.0" {
		t.Fatalf("CodexVersion() = %q, want 0.149.0", got)
	}
}
