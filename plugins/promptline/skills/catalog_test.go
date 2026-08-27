package skills

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

const debianSysadmin = "debian-sysadmin"

func TestEmbeddedCatalogInventory(t *testing.T) {
	catalog := testEmbeddedCatalog(t)

	if got, want := catalog.ListSkills(), []string{debianSysadmin}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ListSkills() = %v, want %v", got, want)
	}
	files, err := catalog.ListFiles(debianSysadmin)
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	want := []string{
		"CHANGELOG.md", "LICENSE", "README.md", "SKILL.md", "agents/openai.yaml",
		"docs/DESIGN.md", "docs/PROVENANCE.md", "docs/UPSTREAM-REVIEW.md",
		"playbooks/disk-full.md", "playbooks/dns-failure.md", "playbooks/failed-boot.md",
		"playbooks/failed-upgrade.md", "playbooks/high-load.md", "playbooks/networking-failure.md",
		"playbooks/package-failure.md", "playbooks/service-failure.md", "playbooks/ssh-failure.md",
		"references/apt-dpkg.md", "references/backups.md", "references/boot-recovery.md",
		"references/diagnostics.md", "references/dns.md", "references/networking.md",
		"references/nftables.md", "references/performance.md", "references/principles.md",
		"references/security.md", "references/shell-safety.md", "references/ssh.md",
		"references/storage.md", "references/systemd.md", "references/toolbox.md",
		"references/users-permissions.md",
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("ListFiles() = %v, want %v", files, want)
	}
	for _, file := range files {
		if strings.HasPrefix(file, "scripts/") || strings.HasPrefix(file, "tests/") {
			t.Fatalf("ListFiles() exposed excluded file %q", file)
		}
	}
	if !sort.StringsAreSorted(files) {
		t.Fatalf("ListFiles() is not sorted: %v", files)
	}
}

func TestEmbeddedRootDiscoversTopLevelSkills(t *testing.T) {
	entries, err := fs.ReadDir(embeddedFiles, ".")
	if err != nil {
		t.Fatalf("read embedded root: %v", err)
	}
	var rootSkills []string
	for _, entry := range entries {
		if !entry.IsDir() || !validName(entry.Name()) {
			continue
		}
		info, err := fs.Stat(embeddedFiles, entry.Name()+"/SKILL.md")
		if err == nil && !info.IsDir() {
			rootSkills = append(rootSkills, entry.Name())
		}
	}
	sort.Strings(rootSkills)

	catalog := testEmbeddedCatalog(t)
	if got := catalog.ListSkills(); !reflect.DeepEqual(got, rootSkills) {
		t.Fatalf("EmbeddedCatalog().ListSkills() = %v, want all embedded-root skills %v", got, rootSkills)
	}
}

func TestCatalogDiscoversFixtureSkillsAndNestedFiles(t *testing.T) {
	fixture := fstest.MapFS{
		"alpha/SKILL.md":             &fstest.MapFile{Data: []byte("alpha")},
		"alpha/references/nested.md": &fstest.MapFile{Data: []byte("nested")},
		"alpha/scripts/check.sh":     &fstest.MapFile{Data: []byte("excluded")},
		"alpha/tests/case.md":        &fstest.MapFile{Data: []byte("excluded")},
		"not-a-skill/file.md":        &fstest.MapFile{Data: []byte("ignore")},
	}
	catalog, err := NewCatalog(fixture)
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	if got, want := catalog.ListSkills(), []string{"alpha"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ListSkills() = %v, want %v", got, want)
	}
	if got, want := mustListFiles(t, catalog, "alpha"), []string{"SKILL.md", "references/nested.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ListFiles() = %v, want %v", got, want)
	}
	bytes, err := catalog.ReadFile("alpha", "references/nested.md")
	if err != nil || string(bytes) != "nested" {
		t.Fatalf("ReadFile() = %q, %v", bytes, err)
	}
}

func TestCatalogReadAndURICanonicalization(t *testing.T) {
	catalog := testEmbeddedCatalog(t)
	bytes, err := catalog.ReadFile(debianSysadmin, "references/systemd.md")
	if err != nil || len(bytes) == 0 {
		t.Fatalf("ReadFile() = %d bytes, %v", len(bytes), err)
	}
	uri, err := catalog.URI(debianSysadmin, "references/systemd.md")
	if err != nil {
		t.Fatalf("URI() error = %v", err)
	}
	if uri != "skill://debian-sysadmin/references/systemd.md" {
		t.Fatalf("URI() = %q", uri)
	}
	skill, file, err := catalog.ParseURI(uri)
	if err != nil || skill != debianSysadmin || file != "references/systemd.md" {
		t.Fatalf("ParseURI() = %q, %q, %v", skill, file, err)
	}
}

func TestCatalogRejectsInvalidPathsAndURIs(t *testing.T) {
	catalog := testEmbeddedCatalog(t)
	invalidFiles := []string{
		"", "../etc/passwd", "../../foo", "/absolute/path", "references/../../../foo",
		"references\\systemd.md", "references//systemd.md", "references/./systemd.md",
		"scripts/validate-skill.sh", "tests/scenarios.md", "references", "missing.md",
	}
	for _, file := range invalidFiles {
		t.Run(file, func(t *testing.T) {
			_, err := catalog.ReadFile(debianSysadmin, file)
			if err == nil {
				t.Fatal("ReadFile() unexpectedly succeeded")
			}
		})
	}
	for _, file := range invalidFiles {
		t.Run("uri/"+file, func(t *testing.T) {
			_, err := catalog.URI(debianSysadmin, file)
			if err == nil {
				t.Fatal("URI() unexpectedly succeeded")
			}
		})
	}
	for _, rawURI := range []string{
		"skill://debian-sysadmin/../etc/passwd", "skill://debian-sysadmin/%2e%2e/etc/passwd",
		"skill://debian-sysadmin/references%2fsystemd.md", "skill://debian-sysadmin/references//systemd.md",
		"skill://debian-sysadmin/references/systemd.md?x=1", "skill://unknown/SKILL.md",
		"skill://debian-sysadmin/scripts/validate-skill.sh", "https://debian-sysadmin/SKILL.md",
	} {
		t.Run(rawURI, func(t *testing.T) {
			_, _, err := catalog.ParseURI(rawURI)
			if err == nil {
				t.Fatal("ParseURI() unexpectedly succeeded")
			}
		})
	}
	if _, err := catalog.ReadFile("missing", "SKILL.md"); !errors.Is(err, ErrUnknownSkill) {
		t.Fatalf("unknown skill error = %v", err)
	}
	if _, err := catalog.ReadFile(debianSysadmin, "missing.md"); !errors.Is(err, ErrUnknownFile) {
		t.Fatalf("unknown file error = %v", err)
	}
}

func TestEmbeddedFilesMatchSource(t *testing.T) {
	catalog := testEmbeddedCatalog(t)
	for _, file := range mustListFiles(t, catalog, debianSysadmin) {
		t.Run(file, func(t *testing.T) {
			got, err := catalog.ReadFile(debianSysadmin, file)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			want, err := os.ReadFile(filepath.Join(debianSysadmin, filepath.FromSlash(file)))
			if err != nil {
				t.Fatalf("read source %q: %v", file, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("embedded bytes differ from source for %q", file)
			}
		})
	}
	for _, file := range []string{"SKILL.md", "references/systemd.md", "playbooks/disk-full.md"} {
		if bytes, err := catalog.ReadFile(debianSysadmin, file); err != nil || len(bytes) == 0 {
			t.Fatalf("explicit embedded read %q = %d bytes, %v", file, len(bytes), err)
		}
	}
}

func TestEmbeddedCatalogWorksOutsideSourceTree(t *testing.T) {
	if os.Getenv("PROMPTLINE_SKILLS_SUBPROCESS") == "1" {
		catalog := testEmbeddedCatalog(t)
		bytes, err := catalog.ReadFile(debianSysadmin, "references/systemd.md")
		if err != nil || len(bytes) == 0 {
			t.Fatalf("embedded subprocess read = %d bytes, %v", len(bytes), err)
		}
		return
	}

	tempDir := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestEmbeddedCatalogWorksOutsideSourceTree$")
	command.Dir = tempDir
	command.Env = append(os.Environ(), "PROMPTLINE_SKILLS_SUBPROCESS=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("subprocess from %q failed: %v\n%s", tempDir, err, output)
	}
	if _, err := fs.Stat(os.DirFS(tempDir), "plugins/promptline/skills"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("temporary cwd unexpectedly has source path: %v", err)
	}
}

func testEmbeddedCatalog(t *testing.T) *Catalog {
	t.Helper()
	catalog, err := EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog() error = %v", err)
	}
	return catalog
}

func mustListFiles(t *testing.T, catalog *Catalog, skill string) []string {
	t.Helper()
	files, err := catalog.ListFiles(skill)
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}
	return files
}
