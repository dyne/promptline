//go:build windows

package instance

import (
	"errors"
	"os"
)

const privateDirectoryMode = 0o700
const privateFileMode = 0o600

// SetPrivateUmaskBeforeConcurrency is a no-op on Windows, where umask is not
// available. Individual files are still created with restrictive modes where
// the platform supports them.
func SetPrivateUmaskBeforeConcurrency() func() { return func() {} }

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
	return os.Chmod(path, privateDirectoryMode)
}
