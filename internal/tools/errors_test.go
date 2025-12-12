package tools

import (
	"errors"
	"testing"
)

func TestToolExecutionError(t *testing.T) {
	baseErr := errors.New("execution failed")
	
	tests := []struct {
		name      string
		err       *ToolExecutionError
		expected  string
	}{
		{
			name: "with operation",
			err: &ToolExecutionError{
				ToolName:  "execute_shell_command",
				Operation: "run",
				Err:       baseErr,
			},
			expected: "tool execute_shell_command failed during run: execution failed",
		},
		{
			name: "without operation",
			err: &ToolExecutionError{
				ToolName: "read_file",
				Err:      baseErr,
			},
			expected: "tool read_file failed: execution failed",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.err.Error())
			}
			
			if !errors.Is(tt.err, baseErr) {
				t.Error("errors.Is should unwrap to base error")
			}
		})
	}
}

func TestPermissionError(t *testing.T) {
	err := &PermissionError{
		ToolName: "write_file",
		Reason:   "tool requires confirmation",
	}
	
	expected := "permission denied for tool write_file: tool requires confirmation"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "ErrToolNotAllowed",
			err:  ErrToolNotAllowed,
			msg:  "tool blocked by policy",
		},
		{
			name: "ErrToolRequiresConfirmation",
			err:  ErrToolRequiresConfirmation,
			msg:  "tool requires confirmation",
		},
		{
			name: "ErrToolNotFound",
			err:  ErrToolNotFound,
			msg:  "tool not found",
		},
		{
			name: "ErrInvalidArguments",
			err:  ErrInvalidArguments,
			msg:  "invalid tool arguments",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("expected %q, got %q", tt.msg, tt.err.Error())
			}
		})
	}
}
