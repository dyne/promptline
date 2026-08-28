package skills

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
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
		"references/caddy.md", "references/diagnostics.md", "references/dns.md", "references/networking.md",
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

func TestCatalogEnforcesCanonicalTextInputs(t *testing.T) {
	tests := []struct {
		name    string
		fixture fstest.MapFS
	}{
		{
			name: "skill name with URI delimiter",
			fixture: fstest.MapFS{
				"alpha?query/SKILL.md": &fstest.MapFile{Data: []byte("skill")},
			},
		},
		{
			name: "file name with URI delimiter",
			fixture: fstest.MapFS{
				"alpha/SKILL.md":        &fstest.MapFile{Data: []byte("skill")},
				"alpha/docs/a?query.md": &fstest.MapFile{Data: []byte("reference")},
			},
		},
		{
			name: "non UTF-8 content",
			fixture: fstest.MapFS{
				"alpha/SKILL.md":   &fstest.MapFile{Data: []byte("skill")},
				"alpha/binary.bin": &fstest.MapFile{Data: []byte{0xff, 0xfe}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewCatalog(tt.fixture); err == nil {
				t.Fatal("NewCatalog() accepted a non-canonical text catalog")
			}
		})
	}
}

func TestCatalogFutureSkillURIsRoundTrip(t *testing.T) {
	fixture := fstest.MapFS{
		"alpha~one/SKILL.md":                &fstest.MapFile{Data: []byte("skill")},
		"alpha~one/references/a_b-1.2~x.md": &fstest.MapFile{Data: []byte("reference")},
	}
	catalog, err := NewCatalog(fixture)
	if err != nil {
		t.Fatal(err)
	}
	const file = "references/a_b-1.2~x.md"
	uri, err := catalog.URI("alpha~one", file)
	if err != nil {
		t.Fatal(err)
	}
	skill, parsedFile, err := catalog.ParseURI(uri)
	if err != nil || skill != "alpha~one" || parsedFile != file {
		t.Fatalf("ParseURI(URI()) = %q, %q, %v", skill, parsedFile, err)
	}
}

func TestCatalogReadRejectsChangedNonUTF8Content(t *testing.T) {
	fixture := fstest.MapFS{
		"alpha/SKILL.md": &fstest.MapFile{Data: []byte("skill")},
	}
	catalog, err := NewCatalog(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fixture["alpha/SKILL.md"].Data = []byte{0xff}
	if _, err := catalog.ReadFile("alpha", "SKILL.md"); !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("ReadFile() error = %v, want ErrInvalidResource", err)
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
	for _, file := range []string{"SKILL.md", "references/caddy.md", "references/systemd.md", "playbooks/disk-full.md"} {
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

func TestCatalogMaterializeReconstructsOnlyPublicFiles(t *testing.T) {
	catalog := testEmbeddedCatalog(t)
	destination := filepath.Join(t.TempDir(), "export")
	if err := catalog.Materialize(destination); err != nil {
		t.Fatal(err)
	}
	files := mustListFiles(t, catalog, debianSysadmin)
	if len(files) != 34 {
		t.Fatalf("public file count = %d, want 34", len(files))
	}
	for _, file := range files {
		want, err := catalog.ReadFile(debianSysadmin, file)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(destination, debianSysadmin, filepath.FromSlash(file)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("materialized %q = %d bytes, %v", file, len(got), err)
		}
	}
	for _, excluded := range []string{"scripts/validate-skill.sh", "tests/scenarios.md"} {
		if _, err := os.Stat(filepath.Join(destination, debianSysadmin, excluded)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("excluded materialized path %q: %v", excluded, err)
		}
	}
}

func TestCatalogMaterializeSupportsNestedMultipleSkillsAndRefusesExistingTargets(t *testing.T) {
	fixture := fstest.MapFS{
		"alpha/SKILL.md":             &fstest.MapFile{Data: []byte("alpha")},
		"alpha/references/nested.md": &fstest.MapFile{Data: []byte("nested")},
		"beta/SKILL.md":              &fstest.MapFile{Data: []byte("beta")},
	}
	catalog, err := NewCatalog(fixture)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "nested", "export")
	if err := catalog.Materialize(destination); err != nil {
		t.Fatal(err)
	}
	if bytes, err := os.ReadFile(filepath.Join(destination, "alpha", "references", "nested.md")); err != nil || string(bytes) != "nested" {
		t.Fatalf("nested materialization = %q, %v", bytes, err)
	}
	if _, err := os.Stat(filepath.Join(destination, "beta", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Materialize(destination); err == nil {
		t.Fatal("materialization overwrote existing skill")
	}
}

func TestCatalogMaterializePreservesUnrelatedDestinationData(t *testing.T) {
	catalog := testEmbeddedCatalog(t)
	destination := t.TempDir()
	unrelated := filepath.Join(destination, "notes.txt")
	if err := os.WriteFile(unrelated, []byte("leave me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Materialize(destination); err != nil {
		t.Fatal(err)
	}
	if bytes, err := os.ReadFile(unrelated); err != nil || string(bytes) != "leave me" {
		t.Fatalf("unrelated destination data = %q, %v", bytes, err)
	}
}

func TestCatalogMaterializeRejectsSymlinkParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	catalog := testEmbeddedCatalog(t)
	base := t.TempDir()
	linked := filepath.Join(base, "linked")
	if err := os.Symlink(t.TempDir(), linked); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Materialize(filepath.Join(linked, "export")); err == nil {
		t.Fatal("materialization followed symlink")
	}
}

func TestMaterializedLocationRejectsTraversal(t *testing.T) {
	for _, file := range []string{"../outside", "/outside", "references/../../outside", "references\\outside"} {
		t.Run(file, func(t *testing.T) {
			if _, _, err := materializedLocation(file); err == nil {
				t.Fatalf("materializedLocation(%q) accepted traversal", file)
			}
		})
	}
}

func TestCatalogMaterializeRejectsNonDirectoryParent(t *testing.T) {
	catalog := testEmbeddedCatalog(t)
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Materialize(filepath.Join(parent, "export")); err == nil {
		t.Fatal("materialization accepted non-directory parent")
	}
}

func TestCatalogMaterializeCleansOwnedStagingAfterWriteFailure(t *testing.T) {
	catalog := testEmbeddedCatalog(t)
	destination := filepath.Join(t.TempDir(), "export")
	original := writeMaterializedFile
	writeMaterializedFile = func(*os.Root, string, []byte) error { return errors.New("injected write failure") }
	t.Cleanup(func() { writeMaterializedFile = original })
	if err := catalog.Materialize(destination); err == nil {
		t.Fatal("materialization succeeded despite injected failure")
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("materialization left staging entries: %v", entries)
	}
}

func TestCatalogMaterializeRejectsDirectorySwapAfterValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated Windows permissions")
	}
	catalog := testEmbeddedCatalog(t)
	parent := t.TempDir()
	destination := filepath.Join(parent, "export")
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	original := beforeOpenMaterializationDirectory
	swapped := false
	beforeOpenMaterializationDirectory = func(_ *os.Root, name string) error {
		if swapped || name != "export" {
			return nil
		}
		swapped = true
		if err := os.Remove(destination); err != nil {
			return err
		}
		return os.Symlink("outside", destination)
	}
	t.Cleanup(func() { beforeOpenMaterializationDirectory = original })
	if err := catalog.Materialize(destination); err == nil {
		t.Fatal("materialization followed a directory swapped to a symlink")
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("materialization wrote through swapped symlink: %v", entries)
	}
}

func TestCatalogMaterializeAtomicallyRefusesRacedTarget(t *testing.T) {
	catalog := testEmbeddedCatalog(t)
	destination := filepath.Join(t.TempDir(), "export")
	original := beforeInstallMaterializedSkill
	created := false
	beforeInstallMaterializedSkill = func(root *os.Root, _, skill string) error {
		if created {
			return nil
		}
		created = true
		return root.Mkdir(skill, 0o755)
	}
	t.Cleanup(func() { beforeInstallMaterializedSkill = original })
	if err := catalog.Materialize(destination); err == nil {
		t.Fatal("materialization replaced a concurrently created target")
	}
	entries, err := os.ReadDir(filepath.Join(destination, debianSysadmin))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("concurrent target was replaced: %v", entries)
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
