package tools

import (
	"errors"
	"fmt"
)

// Common tool errors
var (
	// ErrToolNotAllowed indicates a tool is blocked by the current policy.
	ErrToolNotAllowed = errors.New("tool blocked by policy")

	// ErrToolRequiresConfirmation indicates a tool requires confirmation before running.
	ErrToolRequiresConfirmation = errors.New("tool requires confirmation")

	// ErrToolNotFound indicates the requested tool doesn't exist in the registry.
	ErrToolNotFound = errors.New("tool not found")

	// ErrInvalidArguments indicates tool arguments are invalid or malformed.
	ErrInvalidArguments = errors.New("invalid tool arguments")
)

// ToolExecutionError represents an error during tool execution.
type ToolExecutionError struct {
	ToolName  string
	Operation string
	Err       error
}

func (e *ToolExecutionError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("tool %s failed during %s: %v", e.ToolName, e.Operation, e.Err)
	}
	return fmt.Sprintf("tool %s failed: %v", e.ToolName, e.Err)
}

func (e *ToolExecutionError) Unwrap() error {
	return e.Err
}

// PermissionError represents an error related to tool permissions.
type PermissionError struct {
	ToolName string
	Reason   string
}

func (e *PermissionError) Error() string {
	return fmt.Sprintf("permission denied for tool %s: %s", e.ToolName, e.Reason)
}
