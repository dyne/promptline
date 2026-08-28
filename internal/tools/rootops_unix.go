//go:build !windows

package tools

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
)

func withRootFD(r *os.Root, fn func(int) error) error {
	f, e := r.Open(".")
	if e != nil {
		return e
	}
	defer f.Close()
	return fn(int(f.Fd()))
}
func withParentFD(r *os.Root, n string, fn func(int, string) error) error {
	parts := strings.Split(n, "/")
	if len(parts) == 0 || filepath.IsAbs(n) {
		return fmt.Errorf("noncanonical path")
	}
	return withRootFD(r, func(fd int) error {
		cur := fd
		for _, p := range parts[:len(parts)-1] {
			if p == "" || p == "." || p == ".." {
				return fmt.Errorf("noncanonical path")
			}
			next, e := unix.Openat(cur, p, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
			if e != nil {
				return e
			}
			if cur != fd {
				unix.Close(cur)
			}
			cur = next
		}
		defer func() {
			if cur != fd {
				unix.Close(cur)
			}
		}()
		leaf := parts[len(parts)-1]
		if leaf == "" || leaf == "." || leaf == ".." {
			return fmt.Errorf("noncanonical path")
		}
		return fn(cur, leaf)
	})
}
func rootRename(r *os.Root, a, b string) error {
	return withParentFD(r, a, func(af int, al string) error {
		return withParentFD(r, b, func(bf int, bl string) error { return unix.Renameat(af, al, bf, bl) })
	})
}
func rootRemoveAll(r *os.Root, n string) error {
	return withParentFD(r, n, func(fd int, leaf string) error { return removeAt(fd, leaf) })
}
func removeAt(fd int, n string) error {
	if strings.Contains(n, "..") {
		return fmt.Errorf("noncanonical path")
	}
	if e := unix.Unlinkat(fd, n, 0); e == nil {
		return nil
	}
	d, e := unix.Openat(fd, n, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if e != nil {
		return e
	}
	defer unix.Close(d)
	buf := make([]byte, 8192)
	for {
		z, e := unix.ReadDirent(d, buf)
		if e != nil {
			return e
		}
		if z == 0 {
			break
		}
		_, _, names := unix.ParseDirent(buf[:z], -1, nil)
		for _, child := range names {
			if child == "." || child == ".." {
				continue
			}
			if e := removeAt(d, child); e != nil {
				return e
			}
		}
	}
	return unix.Unlinkat(fd, n, unix.AT_REMOVEDIR)
}
func rootMkdirAll(r *os.Root, n string, m os.FileMode) error {
	parts := strings.Split(n, "/")
	cur := "."
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		cur = filepath.Join(cur, p)
		if e := r.Mkdir(cur, m); e != nil && !os.IsExist(e) {
			return e
		}
	}
	return nil
}
func rootSymlink(r *os.Root, a, b string) error {
	return withParentFD(r, b, func(fd int, leaf string) error { return unix.Symlinkat(a, fd, leaf) })
}
func rootLink(r *os.Root, a, b string) error {
	return withParentFD(r, a, func(af int, al string) error {
		return withParentFD(r, b, func(bf int, bl string) error { return unix.Linkat(af, al, bf, bl, 0) })
	})
}
func rootReadlink(r *os.Root, n string) (string, error) {
	b := make([]byte, 4096)
	var z int
	err := withParentFD(r, n, func(fd int, leaf string) error { var e error; z, e = unix.Readlinkat(fd, leaf, b); return e })
	if err != nil {
		return "", err
	}
	return string(b[:z]), nil
}
