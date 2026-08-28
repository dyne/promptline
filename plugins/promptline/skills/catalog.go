// Package skills provides read-only access to the skill files compiled into
// Promptline.
package skills

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// embeddedFiles includes the complete authoritative source tree. Development
// files remain embedded because go:embed has no exclusion pattern, but they
// are deliberately unreachable through Catalog's public methods.
//
//go:embed *
var embeddedFiles embed.FS

var (
	// ErrInvalidResource identifies a non-canonical skill name, file path, or URI.
	ErrInvalidResource = errors.New("invalid embedded skill resource")
	// ErrUnknownSkill identifies a well-formed skill name absent from the catalog.
	ErrUnknownSkill = errors.New("unknown embedded skill")
	// ErrUnknownFile identifies a well-formed public path absent from a skill.
	ErrUnknownFile = errors.New("unknown embedded skill file")
)

var writeMaterializedFile = func(root *os.Root, name string, bytes []byte) error {
	out, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err = out.Write(bytes); err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

var (
	// These nil production hooks let tests force changes at the two filesystem
	// race boundaries without timing-dependent goroutines.
	beforeOpenMaterializationDirectory func(*os.Root, string) error
	beforeInstallMaterializedSkill     func(*os.Root, string, string) error
)

// Catalog is a discovered, immutable view of public UTF-8 skill files whose
// names have canonical skill URI representations.
type Catalog struct {
	fsys   fs.FS
	files  map[string]map[string]string
	skills []string
}

// EmbeddedCatalog returns a catalog over the skill files compiled into this package.
func EmbeddedCatalog() (*Catalog, error) {
	return NewCatalog(embeddedFiles)
}

// NewCatalog discovers top-level skill directories from fsys. A skill is a
// directory containing SKILL.md; scripts and tests subtrees are excluded.
// Skill names and file path segments are restricted to RFC 3986 unreserved
// ASCII characters so every public file has one canonical skill URI.
func NewCatalog(fsys fs.FS) (*Catalog, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read skill root: %w", err)
	}

	catalog := &Catalog{fsys: fsys, files: map[string]map[string]string{}, skills: []string{}}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skill := entry.Name()
		info, err := fs.Stat(fsys, path.Join(skill, "SKILL.md"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("inspect skill %q: %w", skill, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if !validName(skill) {
			return nil, fmt.Errorf("invalid embedded skill name %q", skill)
		}

		files, err := discoverFiles(fsys, skill)
		if err != nil {
			return nil, err
		}
		catalog.files[skill] = files
		catalog.skills = append(catalog.skills, skill)
	}
	sort.Strings(catalog.skills)
	return catalog, nil
}

func discoverFiles(fsys fs.FS, skill string) (map[string]string, error) {
	files := map[string]string{}
	err := fs.WalkDir(fsys, skill, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == skill {
			return nil
		}
		rel := strings.TrimPrefix(name, skill+"/")
		if excluded(rel) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect embedded path %q: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("embedded path %q is not a regular file", name)
		}
		if !validRelativePath(rel) {
			return fmt.Errorf("invalid embedded path %q", name)
		}
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read embedded path %q: %w", name, err)
		}
		if !utf8.Valid(content) {
			return fmt.Errorf("embedded path %q is not UTF-8 text", name)
		}
		files[rel] = name
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover skill %q: %w", skill, err)
	}
	return files, nil
}

// ListSkills returns sorted embedded skill names.
func (c *Catalog) ListSkills() []string {
	return append([]string{}, c.skills...)
}

// ListFiles returns the sorted public, slash-separated paths for skill.
func (c *Catalog) ListFiles(skill string) ([]string, error) {
	files, err := c.skillFiles(skill)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(files))
	for file := range files {
		paths = append(paths, file)
	}
	sort.Strings(paths)
	return paths, nil
}

// ReadFile returns the exact bytes of one known public embedded skill file.
func (c *Catalog) ReadFile(skill, file string) ([]byte, error) {
	name, err := c.resolve(skill, file)
	if err != nil {
		return nil, err
	}
	bytes, err := fs.ReadFile(c.fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read embedded skill file: %w", err)
	}
	if !utf8.Valid(bytes) {
		return nil, fmt.Errorf("%w: embedded skill file is not UTF-8 text", ErrInvalidResource)
	}
	return bytes, nil
}

// URI returns the canonical URI for an embedded public file.
func (c *Catalog) URI(skill, file string) (string, error) {
	if _, err := c.resolve(skill, file); err != nil {
		return "", err
	}
	return "skill://" + skill + "/" + file, nil
}

// ParseURI resolves a canonical skill URI to its skill name and relative path.
func (c *Catalog) ParseURI(rawURI string) (string, string, error) {
	u, err := url.ParseRequestURI(rawURI)
	if err != nil || u.Scheme != "skill" || u.Host == "" || u.User != nil ||
		u.Port() != "" || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidResource, rawURI)
	}
	file := strings.TrimPrefix(u.Path, "/")
	if !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(file, "/") {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidResource, rawURI)
	}
	if _, err := c.resolve(u.Host, file); err != nil {
		return "", "", err
	}
	return u.Host, file, nil
}

// Materialize writes every public embedded skill below destination. It never
// overwrites a skill directory and rejects symlinks anywhere in the path.
func (c *Catalog) Materialize(destination string) error {
	destinationRoot, absolute, err := openMaterializationDestination(destination)
	if err != nil {
		return err
	}
	defer destinationRoot.Close()

	for _, skill := range c.skills {
		if _, err := destinationRoot.Lstat(skill); err == nil {
			return fmt.Errorf("materialization target already exists: %s", filepath.Join(absolute, skill))
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect materialization target: %w", err)
		}
	}
	staging, err := mkdirTempRoot(destinationRoot, ".promptline-skills-")
	if err != nil {
		return fmt.Errorf("create materialization staging: %w", err)
	}
	defer removeAllRoot(destinationRoot, staging)
	stagingRoot, err := openDirectoryNoFollow(destinationRoot, staging, false, 0)
	if err != nil {
		return fmt.Errorf("open materialization staging: %w", err)
	}
	stagingOpen := true
	defer func() {
		if stagingOpen {
			stagingRoot.Close()
		}
	}()
	for _, skill := range c.skills {
		if err := c.materializeSkill(stagingRoot, skill); err != nil {
			return err
		}
	}
	if err := syncRootDirectory(stagingRoot, "."); err != nil {
		return err
	}
	for _, skill := range c.skills {
		if beforeInstallMaterializedSkill != nil {
			if err := beforeInstallMaterializedSkill(destinationRoot, staging, skill); err != nil {
				return fmt.Errorf("prepare materialized skill installation: %w", err)
			}
		}
		if err := renameNoReplace(stagingRoot, skill, destinationRoot, skill); err != nil {
			return fmt.Errorf("install materialized skill %q: %w", skill, err)
		}
	}
	closeErr := stagingRoot.Close()
	stagingOpen = false
	if closeErr != nil {
		return fmt.Errorf("close materialization staging: %w", closeErr)
	}
	return syncRootDirectory(destinationRoot, ".")
}

func (c *Catalog) materializeSkill(staging *os.Root, skill string) error {
	if err := staging.Mkdir(skill, 0o755); err != nil {
		return fmt.Errorf("create materialized skill %q: %w", skill, err)
	}
	skillRoot, err := openDirectoryNoFollow(staging, skill, false, 0)
	if err != nil {
		return fmt.Errorf("open materialized skill %q: %w", skill, err)
	}
	defer skillRoot.Close()
	files, err := c.ListFiles(skill)
	if err != nil {
		return err
	}
	for _, file := range files {
		directory, name, err := materializedLocation(file)
		if err != nil {
			return err
		}
		bytes, err := c.ReadFile(skill, file)
		if err != nil {
			return err
		}
		parent, err := openRelativeDirectory(skillRoot, directory)
		if err != nil {
			return fmt.Errorf("create materialized directory: %w", err)
		}
		if err := writeMaterializedFile(parent, name, bytes); err != nil {
			parent.Close()
			return fmt.Errorf("write materialized file: %w", err)
		}
		if err := syncRootDirectory(parent, "."); err != nil {
			parent.Close()
			return err
		}
		if err := parent.Close(); err != nil {
			return fmt.Errorf("close materialized directory: %w", err)
		}
	}
	return syncRootDirectory(skillRoot, ".")
}

func materializedLocation(file string) (string, string, error) {
	if !validRelativePath(file) {
		return "", "", fmt.Errorf("invalid materialized file path %q", file)
	}
	local := filepath.FromSlash(file)
	directory, name := filepath.Dir(local), filepath.Base(local)
	if !filepath.IsLocal(local) || name == "." || name == string(filepath.Separator) {
		return "", "", fmt.Errorf("materialized file escapes target: %q", file)
	}
	return directory, name, nil
}

func openMaterializationDestination(destination string) (*os.Root, string, error) {
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return nil, "", fmt.Errorf("resolve materialization destination: %w", err)
	}
	anchor := filepath.VolumeName(absolute) + string(filepath.Separator)
	relative, err := filepath.Rel(anchor, absolute)
	isLocalDestination := relative == "." || filepath.IsLocal(relative)
	if err != nil || !isLocalDestination {
		return nil, "", fmt.Errorf("resolve materialization destination %q", absolute)
	}
	root, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, "", fmt.Errorf("open materialization filesystem root: %w", err)
	}
	if relative == "." {
		return root, absolute, nil
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		next, err := openDirectoryNoFollow(root, component, true, 0o755)
		if err != nil {
			root.Close()
			return nil, "", fmt.Errorf("open materialization destination %q: %w", absolute, err)
		}
		if err := root.Close(); err != nil {
			next.Close()
			return nil, "", fmt.Errorf("close materialization parent: %w", err)
		}
		root = next
	}
	return root, absolute, nil
}

func openRelativeDirectory(root *os.Root, directory string) (*os.Root, error) {
	current, err := root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	if directory == "." {
		return current, nil
	}
	for _, component := range strings.Split(directory, string(filepath.Separator)) {
		next, err := openDirectoryNoFollow(current, component, true, 0o755)
		if err != nil {
			current.Close()
			return nil, err
		}
		if err := current.Close(); err != nil {
			next.Close()
			return nil, err
		}
		current = next
	}
	return current, nil
}

func openDirectoryNoFollow(parent *os.Root, name string, create bool, mode fs.FileMode) (*os.Root, error) {
	info, err := parent.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) && create {
		if err := parent.Mkdir(name, mode); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		info, err = parent.Lstat(name)
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("%q is not a symlink-free directory", name)
	}
	if beforeOpenMaterializationDirectory != nil {
		if err := beforeOpenMaterializationDirectory(parent, name); err != nil {
			return nil, err
		}
	}
	opened, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	openedInfo, openErr := opened.Stat(".")
	currentInfo, currentErr := parent.Lstat(name)
	unchanged := openErr == nil && currentErr == nil && currentInfo.Mode()&os.ModeSymlink == 0
	if !unchanged || !os.SameFile(info, openedInfo) || !os.SameFile(currentInfo, openedInfo) {
		opened.Close()
		return nil, fmt.Errorf("materialization directory %q changed while opening", name)
	}
	return opened, nil
}

func mkdirTempRoot(root *os.Root, prefix string) (string, error) {
	for range 100 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", err
		}
		name := prefix + hex.EncodeToString(random[:])
		if err := root.Mkdir(name, 0o700); err == nil {
			return name, nil
		} else if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", errors.New("could not allocate a unique materialization staging directory")
}

// removeAllRoot removes a command-owned tree using only os.Root operations
// available in Go 1.24. Every recursive directory is opened and identity-
// checked before traversal, so cleanup cannot follow a swapped symlink.
func removeAllRoot(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return root.Remove(name)
	}

	directory, err := openDirectoryNoFollow(root, name, false, 0)
	if err != nil {
		return err
	}
	handle, err := directory.Open(".")
	if err != nil {
		directory.Close()
		return err
	}
	entries, readErr := handle.ReadDir(-1)
	closeHandleErr := handle.Close()
	if readErr != nil {
		directory.Close()
		return readErr
	}
	if closeHandleErr != nil {
		directory.Close()
		return closeHandleErr
	}
	for _, entry := range entries {
		if err := removeAllRoot(directory, entry.Name()); err != nil {
			directory.Close()
			return err
		}
	}
	if err := directory.Close(); err != nil {
		return err
	}
	return root.Remove(name)
}

func syncRootDirectory(root *os.Root, name string) error {
	handle, err := root.Open(name)
	if err != nil {
		return fmt.Errorf("open materialization directory: %w", err)
	}
	err = handle.Sync()
	closeErr := handle.Close()
	if err != nil {
		return fmt.Errorf("sync materialization directory: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close materialization directory: %w", closeErr)
	}
	return nil
}

func (c *Catalog) resolve(skill, file string) (string, error) {
	files, err := c.skillFiles(skill)
	if err != nil {
		return "", err
	}
	if !validRelativePath(file) || excluded(file) {
		return "", fmt.Errorf("%w: %q", ErrInvalidResource, file)
	}
	name, ok := files[file]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownFile, file)
	}
	return name, nil
}

func (c *Catalog) skillFiles(skill string) (map[string]string, error) {
	if !validName(skill) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidResource, skill)
	}
	files, ok := c.files[skill]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownSkill, skill)
	}
	return files, nil
}

func validName(name string) bool {
	return validRelativePath(name) && !strings.Contains(name, "/")
}

func validRelativePath(name string) bool {
	if name == "" || strings.Contains(name, "\\") || !fs.ValidPath(name) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if !validURIPathSegment(part) {
			return false
		}
	}
	return true
}

func validURIPathSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." {
		return false
	}
	for index := range len(segment) {
		character := segment[index]
		lowercase := character >= 'a' && character <= 'z'
		uppercase := character >= 'A' && character <= 'Z'
		letter := lowercase || uppercase
		digit := character >= '0' && character <= '9'
		unreservedPunctuation := strings.ContainsRune("-._~", rune(character))
		if !letter && !digit && !unreservedPunctuation {
			return false
		}
	}
	return true
}

func excluded(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if part == "scripts" || part == "tests" {
			return true
		}
	}
	return false
}
