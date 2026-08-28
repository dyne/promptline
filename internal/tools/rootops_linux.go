//go:build linux

package tools

import "os"
import "golang.org/x/sys/unix"

func rootRenameNoReplace(r *os.Root, a, b string) error {
	return withParentFD(r, a, func(af int, al string) error {
		return withParentFD(r, b, func(bf int, bl string) error { return unix.Renameat2(af, al, bf, bl, unix.RENAME_NOREPLACE) })
	})
}
