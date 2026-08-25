package runtime

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestTerminalFormatsStructuredErrors(t *testing.T) {
	raw := `{"type":"error","status":400,"error":{"type":"invalid_request_error","message":"The model is not supported."}}`
	var output bytes.Buffer
	if err := (Terminal{Out: &output}).Error(errors.New(raw)); err != nil {
		t.Fatal(err)
	}
	want := "error: The model is not supported. (invalid_request_error, HTTP 400)\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestTerminalLeavesPlainErrorsReadable(t *testing.T) {
	var output bytes.Buffer
	if err := (Terminal{Out: &output}).Error(errors.New("plain failure")); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); !strings.Contains(got, "error: plain failure") {
		t.Fatalf("output = %q", got)
	}
}
