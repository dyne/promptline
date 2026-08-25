package runtime

import (
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

func TestDecodeStructuredErrorRejectsPlainText(t *testing.T) {
	if _, ok := decodeStructuredError("plain failure"); ok {
		t.Fatal("plain text was decoded as a structured error")
	}
}
