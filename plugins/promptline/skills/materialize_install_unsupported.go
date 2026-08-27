//go:build !linux && !darwin && !windows

package skills

import (
	"errors"
	"os"
)

func renameNoReplace(*os.Root, string, *os.Root, string) error {
	return errors.New("atomic no-replace materialization is unsupported on this platform")
}
