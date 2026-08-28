//go:build windows

package tools

import (
	"fmt"
	"os"
)

func rootRename(*os.Root, string, string) error {
	return fmt.Errorf("rooted rename unavailable on Windows with Go 1.24")
}
func rootRenameNoReplace(*os.Root, string, string) error {
	return fmt.Errorf("atomic rooted no-replace rename unavailable on Windows with Go 1.24")
}
func rootRemoveAll(*os.Root, string) error {
	return fmt.Errorf("rooted recursive removal unavailable on Windows with Go 1.24")
}
func rootMkdirAll(*os.Root, string, os.FileMode) error {
	return fmt.Errorf("rooted recursive mkdir unavailable on Windows with Go 1.24")
}
func rootSymlink(*os.Root, string, string) error {
	return fmt.Errorf("rooted symlink unavailable on Windows with Go 1.24")
}
func rootLink(*os.Root, string, string) error {
	return fmt.Errorf("rooted link unavailable on Windows with Go 1.24")
}
func rootReadlink(*os.Root, string) (string, error) {
	return "", fmt.Errorf("rooted readlink unavailable on Windows with Go 1.24")
}
