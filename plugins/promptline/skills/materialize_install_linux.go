//go:build linux

package skills

import (
	"os"

	"golang.org/x/sys/unix"
)

func renameNoReplace(oldRoot *os.Root, oldName string, newRoot *os.Root, newName string) error {
	oldDirectory, err := oldRoot.Open(".")
	if err != nil {
		return err
	}
	defer oldDirectory.Close()
	newDirectory, err := newRoot.Open(".")
	if err != nil {
		return err
	}
	defer newDirectory.Close()
	return unix.Renameat2(
		int(oldDirectory.Fd()),
		oldName,
		int(newDirectory.Fd()),
		newName,
		unix.RENAME_NOREPLACE,
	)
}
