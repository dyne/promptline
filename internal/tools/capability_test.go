package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryReadFileUsesLiveRootCapability(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"read_file"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	moved := filepath.Join(parent, "moved")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := registry.ExecuteContext(context.Background(), "read_file", map[string]any{"path": "inside.txt"})
	if result.Error != nil || result.Result != "inside" {
		t.Fatalf("read through moved capability = %#v, want original root content", result)
	}
}

func TestRegistryReadFileRejectsSymlinkEscapeAtOpen(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "sentinel.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "swap")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	registry, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"read_file"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	result := registry.ExecuteContext(context.Background(), "read_file", map[string]any{"path": "swap/sentinel.txt"})
	if result.Error == nil {
		t.Fatalf("symlink escape read succeeded: %#v", result)
	}
}

func TestCatRejectsSymlinkEscapeAtOpen(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "sentinel.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "swap")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	registry, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"cat"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	if result := registry.ExecuteContext(context.Background(), "cat", map[string]any{"path": "swap/sentinel.txt"}); result.Error == nil {
		t.Fatalf("symlink escape cat succeeded: %#v", result)
	}
}

func TestCreateFileDoesNotFollowLeafSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(root, "target.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	registry, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"create_file"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	result := registry.ExecuteContext(context.Background(), "create_file", map[string]any{"path": "target.txt", "content": "changed", "overwrite": true})
	if result.Error != nil {
		t.Fatalf("rooted replacement failed: %#v", result)
	}
	content, err := os.ReadFile(sentinel)
	if err != nil || string(content) != "keep" {
		t.Fatalf("outside sentinel = %q, %v", content, err)
	}
	content, err = os.ReadFile(filepath.Join(root, "target.txt"))
	if err != nil || string(content) != "changed" {
		t.Fatalf("replacement = %q, %v", content, err)
	}
}

func TestTeeDoesNotFollowLeafSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(root, "target.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	registry, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"tee"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	result := registry.ExecuteContext(context.Background(), "tee", map[string]any{"path": "target.txt", "content": "changed"})
	if result.Error != nil {
		t.Fatalf("rooted tee failed: %#v", result)
	}
	content, err := os.ReadFile(sentinel)
	if err != nil || string(content) != "keep" {
		t.Fatalf("outside sentinel = %q, %v", content, err)
	}
}

func TestFindAndGlobRejectSymlinkedOutsideTree(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "sentinel.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "swap")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	registry, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"find", "grep"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	if result := registry.ExecuteContext(context.Background(), "find", map[string]any{"path": "swap"}); result.Error == nil {
		t.Fatalf("find escaped root: %#v", result)
	}
	if result := registry.ExecuteContext(context.Background(), "grep", map[string]any{"path": "swap/*.txt", "pattern": "secret"}); result.Error == nil {
		t.Fatalf("glob escaped root: %#v", result)
	}
}

func TestMoveRequiresExplicitForceToReplace(t *testing.T) {
	root := t.TempDir()
	for name, value := range map[string]string{"source.txt": "source", "destination.txt": "destination"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"mv"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if result := r.ExecuteContext(context.Background(), "mv", map[string]any{"sources": []string{"source.txt"}, "destination": "destination.txt"}); result.Error == nil {
		t.Fatalf("mv replaced without force: %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(root, "destination.txt"))
	if err != nil || string(data) != "destination" {
		t.Fatalf("destination changed: %q %v", data, err)
	}
}

func TestMoveNoReplacePreservesConcurrentDestination(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "destination"), []byte("racer"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"mv"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if got := r.ExecuteContext(context.Background(), "mv", map[string]any{"sources": []string{"source"}, "destination": "destination"}); got.Error == nil {
		t.Fatalf("collision overwrite: %#v", got)
	}
	b, err := os.ReadFile(filepath.Join(root, "destination"))
	if err != nil || string(b) != "racer" {
		t.Fatalf("destination overwritten: %q %v", b, err)
	}
}

func TestRootedMutatorsRejectOutsideSentinel(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	r, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"mkdir", "touch", "truncate", "rm"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, tc := range []struct {
		name string
		args map[string]any
	}{{"mkdir", map[string]any{"path": "escape/new"}}, {"touch", map[string]any{"path": "escape/new"}}, {"truncate", map[string]any{"path": "escape/sentinel", "size": 0}}, {"rm", map[string]any{"path": "escape/sentinel"}}} {
		if got := r.ExecuteContext(context.Background(), tc.name, tc.args); got.Error == nil {
			t.Fatalf("%s escaped: %#v", tc.name, got)
		}
	}
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "keep" {
		t.Fatalf("sentinel changed: %q %v", data, err)
	}
}

func TestCopyMoveAndLinkRejectOutsideSentinel(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	r, err := NewRegistryWithConfig(Config{WorkingDirectory: root, Roots: []string{root}, Policy: PolicyFromLists([]string{"cp", "mv", "ln"}, nil, nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, tc := range []struct {
		name string
		args map[string]any
	}{{"cp", map[string]any{"sources": []string{"source"}, "destination": "escape/x", "force": true}}, {"mv", map[string]any{"sources": []string{"source"}, "destination": "escape/x", "force": true}}, {"ln", map[string]any{"target": "source", "link_path": "escape/x"}}} {
		if got := r.ExecuteContext(context.Background(), tc.name, tc.args); got.Error == nil {
			t.Fatalf("%s escaped: %#v", tc.name, got)
		}
	}
}
