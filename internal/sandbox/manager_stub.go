//go:build !linux

package sandbox

import (
	"context"
	"fmt"
	"io"

	"github.com/criyle/go-sandbox/runner"
	"promptline/internal/config"
)

// Manager is a stub for non-Linux platforms.
type Manager struct {
	cfg config.Sandbox
}

// NewManager constructs a stub manager.
func NewManager(cfg config.Sandbox) *Manager {
	return &Manager{cfg: cfg}
}

// Start is a no-op on non-Linux platforms.
func (m *Manager) Start() error {
	return fmt.Errorf("sandbox not supported on this platform")
}

// ExecInSandbox is unsupported on non-Linux platforms.
func (m *Manager) ExecInSandbox(_ context.Context, _ string, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) (runner.Result, error) {
	return runner.Result{}, fmt.Errorf("sandbox not supported on this platform")
}

// Close is a no-op on non-Linux platforms.
func (m *Manager) Close() error {
	return nil
}
