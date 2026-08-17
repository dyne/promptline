package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDirInCwd(t *testing.T) (string, string) {
	t.Helper()
	base, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	dir, err := os.MkdirTemp(".", "tools-test-")
	if err != nil {
		t.Fatalf("create temporary directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("resolve temporary directory: %v", err)
	}
	relDir, err := filepath.Rel(base, absDir)
	if err != nil {
		t.Fatalf("make temporary directory relative: %v", err)
	}
	return absDir, relDir
}
