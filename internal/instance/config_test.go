package instance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstanceNameValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{"alpha", true}, {"a-1", true}, {"a123", true}, {"", false}, {"-alpha", false},
		{"alpha/child", false}, {"alpha\\child", false}, {"..", false}, {"a_", false}, {"café", false}, {"a\x00b", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if instanceNamePattern.MatchString(tc.name) != tc.valid {
				t.Fatalf("validity mismatch for %q", tc.name)
			}
		})
	}
}

func TestResolveStateRootByPrivilege(t *testing.T) {
	if got, err := resolveStateRoot(0, ""); err != nil || got != defaultRootStateDirectory {
		t.Fatalf("root default = %q, %v", got, err)
	}
	if _, err := resolveStateRoot(1000, ""); err == nil {
		t.Fatal("non-root default should fail")
	}
	if got, err := resolveStateRoot(1000, "/tmp/promptline-state"); err != nil || got != "/tmp/promptline-state" {
		t.Fatalf("explicit root = %q, %v", got, err)
	}
	if _, err := resolveStateRoot(1000, "relative"); err == nil {
		t.Fatal("relative root should fail")
	}
}

func TestNewIsolatesImmutableInstances(t *testing.T) {
	root := t.TempDir()
	work := t.TempDir()
	plugins := []string{"plugin-a"}
	one, err := New(Config{Name: "one", StateRoot: root, WorkingRoot: work, PluginPassthrough: plugins})
	if err != nil {
		t.Fatal(err)
	}
	two, err := New(Config{Name: "two", StateRoot: root, WorkingRoot: work})
	if err != nil {
		t.Fatal(err)
	}
	if one.StateDir() == two.StateDir() || filepath.Dir(one.StateDir()) != root {
		t.Fatal("instances are not isolated")
	}
	plugins[0] = "changed"
	got := one.PluginPassthrough()
	got[0] = "changed-again"
	if one.PluginPassthrough()[0] != "plugin-a" {
		t.Fatal("instance retained mutable configuration")
	}
	if one.ApprovalMode() != ApprovalDeny || one.Timeouts().Startup <= 0 || one.OutputCaps().StdoutBytes <= 0 {
		t.Fatal("defaults are not conservative")
	}
	for _, path := range []string{root, one.StateDir(), one.CodexHome()} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != privateDirectoryMode {
			t.Fatalf("%s mode = %o", path, info.Mode().Perm())
		}
	}
}

func TestNewRejectsUnsafeRootsAndWorkingDirectories(t *testing.T) {
	work := t.TempDir()
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []Config{
		{Name: "one", StateRoot: link, WorkingRoot: work},
		{Name: "one", StateRoot: root, WorkingRoot: work, WorkingDirectory: t.TempDir()},
		{Name: "one", StateRoot: root, WorkingRoot: work, ApprovalMode: "unsafe"},
	} {
		if _, err := New(cfg); err == nil {
			t.Fatalf("New(%+v) unexpectedly succeeded", cfg)
		}
	}
}

func TestDecodeConfigRejectsUnknownFields(t *testing.T) {
	if _, err := DecodeConfig([]byte(`{"Name":"one","bogus":true}`)); err == nil {
		t.Fatal("unknown configuration field accepted")
	}
	if _, err := DecodeConfig([]byte(`{"Name":"one"} {}`)); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
}
