// Package skills provides read-only access to the skill files compiled into
// Promptline.
package skills

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
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

var writeMaterializedFile = func(path string, bytes []byte) error {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
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

// Catalog is a discovered, immutable view of public skill files.
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
func NewCatalog(fsys fs.FS) (*Catalog, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read skill root: %w", err)
	}

	catalog := &Catalog{fsys: fsys, files: map[string]map[string]string{}, skills: []string{}}
	for _, entry := range entries {
		if !entry.IsDir() || !validName(entry.Name()) {
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
		if info.IsDir() {
			continue
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
		if !validRelativePath(rel) {
			return fmt.Errorf("invalid embedded path %q", name)
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
	root, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve materialization destination: %w", err)
	}
	if err := requireDirectoryTree(filepath.Dir(root)); err != nil {
		return err
	}
	if info, err := os.Lstat(root); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("materialization destination %q is not a directory", root)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect materialization destination: %w", err)
	}

	for _, skill := range c.skills {
		target := filepath.Join(root, skill)
		if filepath.Dir(target) != root {
			return fmt.Errorf("invalid materialization target %q", skill)
		}
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("materialization target already exists: %s", target)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect materialization target: %w", err)
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create materialization destination: %w", err)
	}
	staging, err := os.MkdirTemp(root, ".promptline-skills-")
	if err != nil {
		return fmt.Errorf("create materialization staging: %w", err)
	}
	defer os.RemoveAll(staging)
	for _, skill := range c.skills {
		if err := c.materializeSkill(staging, skill); err != nil {
			return err
		}
	}
	for _, skill := range c.skills {
		if err := os.Rename(filepath.Join(staging, skill), filepath.Join(root, skill)); err != nil {
			return fmt.Errorf("install materialized skill %q: %w", skill, err)
		}
	}
	return syncDirectory(root)
}

func (c *Catalog) materializeSkill(staging, skill string) error {
	target := filepath.Join(staging, skill)
	if err := os.Mkdir(target, 0o755); err != nil {
		return fmt.Errorf("create materialized skill %q: %w", skill, err)
	}
	files, err := c.ListFiles(skill)
	if err != nil {
		return err
	}
	for _, file := range files {
		path, err := materializedPath(target, file)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create materialized directory: %w", err)
		}
		bytes, err := c.ReadFile(skill, file)
		if err != nil {
			return err
		}
		if err := writeMaterializedFile(path, bytes); err != nil {
			return fmt.Errorf("write materialized file: %w", err)
		}
	}
	return syncDirectory(target)
}

func materializedPath(root, file string) (string, error) {
	if !validRelativePath(file) {
		return "", fmt.Errorf("invalid materialized file path %q", file)
	}
	path := filepath.Join(root, filepath.FromSlash(file))
	if relative, err := filepath.Rel(root, path); err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("materialized file escapes target: %q", file)
	}
	return path, nil
}

func requireDirectoryTree(directory string) error {
	for current := directory; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("materialization parent %q is not a directory", current)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect materialization parent: %w", err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}

func syncDirectory(directory string) error {
	handle, err := os.Open(directory)
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
		if part == "" || part == "." || part == ".." {
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
