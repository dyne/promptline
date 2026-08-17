//go:build unix

package instance

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

const privateDirectoryMode = 0o700
const privateFileMode = 0o600

// SetPrivateUmaskBeforeConcurrency must be called by the composition root
// before it starts goroutines or creates artifacts. It returns a restore
// function for tests and short-lived setup paths. Instance creation also sets
// explicit modes, so it never mutates this process-global setting itself.
func SetPrivateUmaskBeforeConcurrency() func() {
	previous := syscall.Umask(0o077)
	return func() { syscall.Umask(previous) }
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(path, privateDirectoryMode); err != nil {
			return err
		}
		info, err = os.Lstat(path)
		if err != nil {
			return err
		}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("must not be a symlink")
	}
	if !info.IsDir() {
		return errors.New("is not a directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("cannot determine directory owner")
	}
	if int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("ownership mismatch: uid %d", stat.Uid)
	}
	if err := os.Chmod(path, privateDirectoryMode); err != nil {
		return err
	}
	return nil
}
