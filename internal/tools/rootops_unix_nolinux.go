//go:build !windows && !linux

package tools

import (
	"fmt"
	"os"
)

func rootRenameNoReplace(*os.Root, string, string) error {
	return fmt.Errorf("atomic rooted no-replace rename unavailable on this platform")
}
