//go:build !windows

package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootedOperationsAreNoFollowAndRecursive(t *testing.T) {
	base := t.TempDir()
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := rootMkdirAll(root, "a/b", 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := root.OpenFile("a/b/file", os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if err := rootRename(root, "a/b/file", "a/b/moved"); err != nil {
		t.Fatal(err)
	}
	if err := rootLink(root, "a/b/moved", "a/b/hard"); err != nil {
		t.Fatal(err)
	}
	if err := rootSymlink(root, "moved", "a/b/link"); err != nil {
		t.Fatal(err)
	}
	if target, err := rootReadlink(root, "a/b/link"); err != nil || target != "moved" {
		t.Fatalf("link=%q %v", target, err)
	}
	if err := rootRemoveAll(root, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Lstat("a"); !os.IsNotExist(err) {
		t.Fatalf("tree remains: %v", err)
	}
	if err := rootRemoveAll(root, "../escape"); err == nil {
		t.Fatal("traversal accepted")
	}
	if err := rootLink(root, "a/b/missing", "a/b/other"); err == nil {
		t.Fatal("missing hardlink source accepted")
	}
	if _, err := rootReadlink(root, "a/b/missing"); err == nil {
		t.Fatal("missing symlink accepted")
	}
	if err := rootRemoveAll(root, "missing"); err == nil {
		t.Fatal("missing removal accepted")
	}
}

func TestCapabilityPathAndReadBounds(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "file"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	capabilities, err := openRootCapabilities([]string{base}, base)
	if err != nil {
		t.Fatal(err)
	}
	defer capabilities[0].root.Close()
	config := Config{WorkingDirectory: base, capabilities: capabilities}
	path, err := capabilityPathFor("file", config)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := readCapabilityFile(path, 3); err == nil {
		t.Fatal("oversized file accepted")
	}
	if _, err := capabilityPathFor("../escape", config); err == nil {
		t.Fatal("outside capability accepted")
	}
	destination, err := capabilityPathFor("copy", config)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyCapabilityTree(path, destination, false, false, 64); err != nil {
		t.Fatal(err)
	}
	if data, _, err := readCapabilityFile(destination, 64); err != nil || string(data) != "content" {
		t.Fatalf("copy=%q %v", data, err)
	}
	if err := replaceCapabilityFile(destination, []byte("updated"), 0o600, true); err != nil {
		t.Fatal(err)
	}
	if data, _, err := readCapabilityFile(destination, 64); err != nil || string(data) != "updated" {
		t.Fatalf("replace=%q %v", data, err)
	}
	if err := replaceCapabilityFile(capabilityPath{capability: capabilities[0], name: "."}, []byte("x"), 0o600, true); err == nil {
		t.Fatal("root replacement accepted")
	}
	if err := os.Mkdir(filepath.Join(base, "tree"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "tree", "child"), []byte("tree"), 0o600); err != nil {
		t.Fatal(err)
	}
	tree, err := capabilityPathFor("tree", config)
	if err != nil {
		t.Fatal(err)
	}
	treeCopy, err := capabilityPathFor("tree-copy", config)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyCapabilityTree(tree, treeCopy, true, false, 64); err != nil {
		t.Fatal(err)
	}
	if data, _, err := readCapabilityFile(capabilityPath{capability: capabilities[0], name: "tree-copy/child"}, 64); err != nil || string(data) != "tree" {
		t.Fatalf("tree copy=%q %v", data, err)
	}
	if err := os.Symlink("child", filepath.Join(base, "tree", "link")); err != nil {
		t.Fatal(err)
	}
	if err := copyCapabilityTree(tree, treeCopy, true, true, 64); err == nil {
		t.Fatal("symlinked source accepted")
	}
	if err := copyCapabilityTree(tree, treeCopy, false, false, 64); err == nil {
		t.Fatal("directory copy without recursive accepted")
	}
}
