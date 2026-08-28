package tools

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"promptline/internal/paths"
	"strings"
)

func copyCapabilityTree(src, dst capabilityPath, recursive bool, force bool, limit int64) error {
	info, err := src.capability.root.Lstat(src.name)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symbolic link source")
	}
	if !info.IsDir() {
		data, info, err := readCapabilityFile(src, limit)
		if err != nil {
			return err
		}
		return replaceCapabilityFile(dst, data, info.Mode().Perm(), force)
	}
	if !recursive {
		return fmt.Errorf("source is a directory (set recursive to true)")
	}
	return fs.WalkDir(src.capability.root.FS(), src.name, func(path string, e fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src.name, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst.name, rel)
		if e.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("refusing symbolic link source")
		}
		if e.IsDir() {
			return rootMkdirAll(dst.capability.root, target, info.Mode().Perm())
		}
		if !e.Type().IsRegular() {
			return fmt.Errorf("refusing nonregular source")
		}
		data, fi, err := readCapabilityFile(capabilityPath{src.capability, path}, limit)
		if err != nil {
			return err
		}
		return replaceCapabilityFile(capabilityPath{dst.capability, target}, data, fi.Mode().Perm(), force)
	})
}

// rootCapability is an opened, descriptor-rooted authority. rootPath is only
// retained for rendering and for translating an explicitly supplied absolute
// path to the canonical name accepted by os.Root; it is never used for I/O.
type rootCapability struct {
	rootPath string
	root     *os.Root
}

type capabilityPath struct {
	capability rootCapability
	name       string
}

func openRootCapabilities(roots []string, workingDirectory string) ([]rootCapability, error) {
	capabilities := make([]rootCapability, 0, len(roots))
	for _, rootPath := range roots {
		root, err := os.OpenRoot(rootPath)
		if err != nil {
			for _, capability := range capabilities {
				_ = capability.root.Close()
			}
			return nil, fmt.Errorf("open toolbox root %q: %w", rootPath, err)
		}
		capabilities = append(capabilities, rootCapability{rootPath: rootPath, root: root})
	}
	if len(capabilities) == 0 {
		return nil, fmt.Errorf("toolbox has no filesystem capability for %q", workingDirectory)
	}
	return capabilities, nil
}

func capabilityPathFor(ctxPath string, config Config) (capabilityPath, error) {
	if err := paths.ValidatePathString(ctxPath, maxPathLength); err != nil {
		return capabilityPath{}, err
	}
	var absolute string
	if filepath.IsAbs(ctxPath) {
		absolute = filepath.Clean(ctxPath)
	} else {
		absolute = filepath.Clean(filepath.Join(config.WorkingDirectory, ctxPath))
	}
	for _, capability := range config.capabilities {
		rel, err := filepath.Rel(capability.rootPath, absolute)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		if rel == "." {
			rel = "."
		}
		return capabilityPath{capability: capability, name: rel}, nil
	}
	return capabilityPath{}, fmt.Errorf("path is outside allowed tool base directories")
}

func readCapabilityFile(path capabilityPath, maxBytes int64) ([]byte, os.FileInfo, error) {
	file, err := path.capability.root.Open(path.name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() || !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("path %q is not a regular file", path.name)
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil, nil, fmt.Errorf("file exceeds maximum size of %d bytes", maxBytes)
	}
	limit := maxBytes
	if limit <= 0 {
		limit = info.Size()
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, nil, fmt.Errorf("file exceeds maximum size of %d bytes", maxBytes)
	}
	return data, info, nil
}

// replaceCapabilityFile writes a same-directory temporary file and renames it
// through the root capability. The destination is never opened by pathname.
func replaceCapabilityFile(path capabilityPath, data []byte, mode os.FileMode, overwrite bool) error {
	if path.name == "." {
		return fmt.Errorf("cannot replace a capability root")
	}
	parent := filepath.Dir(path.name)
	if parent != "." {
		if err := rootMkdirAll(path.capability.root, parent, 0o755); err != nil {
			return err
		}
	}
	if !overwrite {
		file, err := path.capability.root.OpenFile(path.name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(data)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil {
			_ = path.capability.root.Remove(path.name)
			return writeErr
		}
		return closeErr
	}
	base := filepath.Base(path.name)
	temporary := filepath.Join(parent, "."+base+".promptline-tmp")
	file, err := path.capability.root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = path.capability.root.Remove(temporary)
		}
	}()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := rootRename(path.capability.root, temporary, path.name); err != nil {
		return err
	}
	committed = true
	return nil
}
