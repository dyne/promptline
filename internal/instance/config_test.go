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
	home := t.TempDir()
	if got, err := resolveStateRoot(0, "", ""); err != nil || got != defaultRootStateDirectory {
		t.Fatalf("root default = %q, %v", got, err)
	}
	wantUserRoot := filepath.Join(home, defaultUserStateDirectory)
	if got, err := resolveStateRoot(1000, "", home); err != nil || got != wantUserRoot {
		t.Fatalf("user default = %q, %v; want %q", got, err, wantUserRoot)
	}
	if _, err := resolveStateRoot(1000, "", ""); err == nil {
		t.Fatal("missing home directory accepted for user default")
	}
	if got, err := resolveStateRoot(1000, "/tmp/promptline-state", ""); err != nil || got != "/tmp/promptline-state" {
		t.Fatalf("explicit root = %q, %v", got, err)
	}
	if _, err := resolveStateRoot(1000, "relative", ""); err == nil {
		t.Fatal("relative root should fail")
	}
}

func TestNewCreatesMissingStateRoot(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "nested", "instances")
	in, err := New(Config{Name: "one", StateRoot: stateRoot, WorkingRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{stateRoot, in.StateDir(), in.CodexHome()} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != privateDirectoryMode {
			t.Fatalf("%s mode = %o", path, info.Mode().Perm())
		}
	}
}

func TestNewIsolatesImmutableInstances(t *testing.T) {
	root := t.TempDir()
	work := t.TempDir()
	one, err := New(Config{Name: "one", StateRoot: root, WorkingRoot: work})
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
	if one.ApprovalMode() != ApprovalDeny || one.Timeouts().Startup <= 0 || one.OutputCaps().StdoutBytes <= 0 {
		t.Fatal("defaults are not conservative")
	}
	if one.Model() != DefaultModel {
		t.Fatalf("default model = %q, want %q", one.Model(), DefaultModel)
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
