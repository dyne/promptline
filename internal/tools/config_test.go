package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRegistryWithConfig_IsolatedAndImmutable(t *testing.T) {
	t.Parallel()
	leftRoot := t.TempDir()
	rightRoot := t.TempDir()
	leftResolved, err := filepath.EvalSymlinks(leftRoot)
	if err != nil {
		t.Fatal(err)
	}
	rightResolved, err := filepath.EvalSymlinks(rightRoot)
	if err != nil {
		t.Fatal(err)
	}
	left, err := NewRegistryWithConfig(Config{WorkingDirectory: leftRoot, Roots: []string{leftRoot}, Limits: Limits{MaxFileSizeBytes: 101}, RateLimits: RateLimitConfig{DefaultPerMinute: 1}, Timeouts: TimeoutConfig{Default: time.Second}, Policy: PolicyFromLists([]string{"get_current_datetime"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = left.Close() })
	right, err := NewRegistryWithConfig(Config{WorkingDirectory: rightRoot, Roots: []string{rightRoot}, Limits: Limits{MaxFileSizeBytes: 202}, RateLimits: RateLimitConfig{DefaultPerMinute: 2}, Timeouts: TimeoutConfig{Default: 2 * time.Second}, Policy: PolicyFromLists(nil, []string{"get_current_datetime"}, nil)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = right.Close() })

	leftConfig, rightConfig := left.Config(), right.Config()
	if leftConfig.Limits.MaxFileSizeBytes != 101 || rightConfig.Limits.MaxFileSizeBytes != 202 {
		t.Fatalf("configs crossed: left=%d right=%d", leftConfig.Limits.MaxFileSizeBytes, rightConfig.Limits.MaxFileSizeBytes)
	}
	if leftConfig.Roots[0] != leftResolved || rightConfig.Roots[0] != rightResolved {
		t.Fatalf("roots crossed: left=%q right=%q", leftConfig.Roots, rightConfig.Roots)
	}
	leftConfig.Roots[0] = rightResolved
	if left.Config().Roots[0] != leftResolved {
		t.Fatal("returned config mutated registry")
	}
	if left.getPermission("get_current_datetime").Level != PermissionAllow || right.getPermission("get_current_datetime").Level != PermissionAsk {
		t.Fatal("policy crossed between registries")
	}
}

func TestRegistryExecuteContext_CancellationAndClose(t *testing.T) {
	t.Parallel()
	registry := NewRegistry()
	t.Cleanup(func() { _ = registry.Close() })
	if err := registry.RegisterTool(&ToolDefinition{NameValue: "context", VersionValue: "1", ExecuteFunc: func(ctx context.Context, _ map[string]interface{}) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}); err != nil {
		t.Fatal(err)
	}
	registry.AllowTool("context", false)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result := registry.ExecuteContext(ctx, "context", nil)
	if !errors.Is(result.Error, context.Canceled) || !errors.Is(result.Error, ErrToolCancelled) {
		t.Fatalf("error = %v, want cancellation", result.Error)
	}
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryFilesystemToolsUseConfiguredWorkingDirectoryAndRoots(t *testing.T) {
	root := t.TempDir()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"read_file", "pwd"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })

	if result := registry.ExecuteContext(context.Background(), "read_file", map[string]any{"path": "inside.txt"}); result.Error != nil || result.Result != "inside" {
		t.Fatalf("configured-root read = %#v", result)
	}
	if result := registry.ExecuteContext(context.Background(), "read_file", map[string]any{"path": filepath.Join(outside, "outside.txt")}); result.Error == nil {
		t.Fatal("read outside configured roots succeeded")
	}
	if result := registry.ExecuteContext(context.Background(), "pwd", map[string]any{}); result.Error != nil || result.Result != resolvedRoot {
		t.Fatalf("configured working directory = %#v, want %q", result, resolvedRoot)
	}
}

func TestRegistryExecuteContext_Timeout(t *testing.T) {
	t.Parallel()
	registry, err := NewRegistryWithConfig(Config{WorkingDirectory: t.TempDir(), Timeouts: TimeoutConfig{Default: time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })
	if err := registry.RegisterTool(&ToolDefinition{NameValue: "timeout", VersionValue: "1", ExecuteFunc: func(ctx context.Context, _ map[string]interface{}) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}); err != nil {
		t.Fatal(err)
	}
	registry.AllowTool("timeout", false)
	result := registry.ExecuteContext(t.Context(), "timeout", nil)
	if !errors.Is(result.Error, context.DeadlineExceeded) || !errors.Is(result.Error, ErrToolTimeout) {
		t.Fatalf("error = %v, want timeout", result.Error)
	}
}

func TestRegistryRoots_RejectPrefixCollisionAndSymlinkEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	escape := t.TempDir()
	if err := os.Symlink(escape, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	registry, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"read_file"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })
	if err := os.WriteFile(filepath.Join(escape, "secret"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := registry.ExecuteContext(t.Context(), "read_file", map[string]interface{}{"path": "escape/secret"}); result.Error == nil {
		t.Fatal("symlink escape was allowed")
	}
}

func TestURootScopedRoots_RejectOutsideAndCrossRootPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	otherRoot := t.TempDir()
	registry, err := NewRegistryWithConfig(Config{
		WorkingDirectory: root,
		Roots:            []string{root},
		Policy:           PolicyFromLists([]string{"cat", "cp"}, nil, nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })

	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := registry.ExecuteContext(t.Context(), "cat", map[string]interface{}{"path": "inside.txt"}); result.Error != nil {
		t.Fatalf("read inside root: %v", result.Error)
	}
	if result := registry.ExecuteContext(t.Context(), "cat", map[string]interface{}{"path": filepath.Join(otherRoot, "outside.txt")}); result.Error == nil {
		t.Fatal("absolute path outside selected root was allowed")
	}
	if result := registry.ExecuteContext(t.Context(), "cp", map[string]interface{}{
		"sources":     []string{"inside.txt"},
		"destination": filepath.Join(otherRoot, "copy.txt"),
	}); result.Error == nil {
		t.Fatal("cross-root copy was allowed")
	}
}
