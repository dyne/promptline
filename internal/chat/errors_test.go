package chat

import (
	"errors"
	"testing"
)

func TestStreamError(t *testing.T) {
	baseErr := errors.New("connection failed")
	err := &StreamError{Operation: "receive", Err: baseErr}
	
	expected := "streaming error during receive: connection failed"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
	
	if !errors.Is(err, baseErr) {
		t.Error("errors.Is should unwrap to base error")
	}
}

func TestToolExecutionError(t *testing.T) {
	baseErr := errors.New("file not found")
	err := &ToolExecutionError{ToolName: "read_file", Err: baseErr}
	
	expected := "tool execution error for read_file: file not found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
	
	if !errors.Is(err, baseErr) {
		t.Error("errors.Is should unwrap to base error")
	}
}

func TestAPIError(t *testing.T) {
	baseErr := errors.New("rate limit exceeded")
	err := &APIError{Operation: "create_completion", Err: baseErr}
	
	expected := "API error during create_completion: rate limit exceeded"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
	
	if !errors.Is(err, baseErr) {
		t.Error("errors.Is should unwrap to base error")
	}
}

func TestHistoryError(t *testing.T) {
	baseErr := errors.New("permission denied")
	err := &HistoryError{Operation: "open", Filepath: "/tmp/test.json", Err: baseErr}
	
	expected := "history error during open on /tmp/test.json: permission denied"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
	
	if !errors.Is(err, baseErr) {
		t.Error("errors.Is should unwrap to base error")
	}
}
