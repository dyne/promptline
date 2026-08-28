package runtime

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestDecodeStructuredErrorPreservesErrorCategory(t *testing.T) {
	raw := `{"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The model is not supported."}}`
	detail, ok := decodeStructuredError(raw)
	if !ok {
		t.Fatal("structured error was not decoded")
	}
	if detail.kind != "invalid_request_error" || detail.status != "400" {
		t.Fatalf("decoded category = kind %q, status %q", detail.kind, detail.status)
	}
}

func TestTerminalSanitizesControlsAcrossAllOutputKinds(t *testing.T) {
	var out bytes.Buffer
	term := Terminal{Out: &out}
	unsafe := "ok\x1b[2J\r\b\x00\u202E"
	_ = term.Delta(unsafe)
	_ = term.Text(unsafe)
	_ = term.Progress(unsafe)
	_ = term.Error(errors.New(unsafe))
	if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "\u202e") {
		t.Fatalf("unsafe output: %q", out.String())
	}
}

func TestDecodeStructuredErrorRejectsPlainText(t *testing.T) {
	if _, ok := decodeStructuredError("plain failure"); ok {
		t.Fatal("plain text was decoded as a structured error")
	}
}
