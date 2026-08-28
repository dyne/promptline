//go:build !windows

package governance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// rootedRename is Go 1.24-compatible: it opens each parent beneath the live
// Root descriptor and invokes renameat without reconstructing a host path.
func rootedRename(root *os.Root, old, next string) error {
	return rootedParent(root, old, func(oldFD int, oldLeaf string) error {
		return rootedParent(root, next, func(nextFD int, nextLeaf string) error {
			return unix.Renameat(oldFD, oldLeaf, nextFD, nextLeaf)
		})
	})
}

func rootedParent(root *os.Root, name string, fn func(int, string) error) error {
	parts := strings.Split(filepath.ToSlash(name), "/")
	if len(parts) == 0 || filepath.IsAbs(name) {
		return fmt.Errorf("noncanonical rooted name")
	}
	base, err := root.Open(".")
	if err != nil {
		return err
	}
	defer base.Close()
	fd := int(base.Fd())
	current := fd
	for _, part := range parts[:len(parts)-1] {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("noncanonical rooted name")
		}
		next, err := unix.Openat(current, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
		if err != nil {
			return err
		}
		if current != fd {
			_ = unix.Close(current)
		}
		current = next
	}
	if current != fd {
		defer unix.Close(current)
	}
	leaf := parts[len(parts)-1]
	if leaf == "" || leaf == "." || leaf == ".." {
		return fmt.Errorf("noncanonical rooted name")
	}
	return fn(current, leaf)
}

func rootedOpenJournalFile(root *os.Root, name string) (*os.File, error) {
	var result *os.File
	err := rootedParent(root, name, func(parent int, leaf string) error {
		fd, err := unix.Openat(parent, leaf, unix.O_CREAT|unix.O_APPEND|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
		if err != nil {
			return err
		}
		result = os.NewFile(uintptr(fd), leaf)
		if result == nil {
			_ = unix.Close(fd)
			return errors.New("create rooted audit file")
		}
		return nil
	})
	return result, err
}

func rootedOpenJournalDir(root *os.Root, name string) (*os.File, error) {
	var result *os.File
	err := rootedParent(root, name, func(parent int, leaf string) error {
		if err := unix.Mkdirat(parent, leaf, 0o700); err != nil && err != unix.EEXIST {
			return err
		}
		fd, err := unix.Openat(parent, leaf, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		var st unix.Stat_t
		if err := unix.Fstat(fd, &st); err != nil {
			_ = unix.Close(fd)
			return err
		}
		if st.Uid != uint32(os.Geteuid()) || st.Nlink != 2 || st.Mode&0o077 != 0 {
			_ = unix.Close(fd)
			return errors.New("unsafe rooted audit directory")
		}
		result = os.NewFile(uintptr(fd), leaf)
		if result == nil {
			_ = unix.Close(fd)
			return errors.New("open rooted audit directory")
		}
		return nil
	})
	return result, err
}
