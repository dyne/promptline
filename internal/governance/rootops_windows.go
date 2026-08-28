//go:build windows

package governance

import (
	"errors"
	"os"
)

// Go 1.24 has no Root.Rename on Windows. Keep the rooted journal fail-closed
// rather than falling back to a host pathname during rotation.
func rootedRename(*os.Root, string, string) error {
	return errors.New("rooted audit rotation is unavailable on this platform")
}

func rootedOpenJournalFile(*os.Root, string) (*os.File, error) {
	return nil, errors.New("rooted audit open is unavailable on this platform")
}
func rootedOpenJournalDir(*os.Root, string) (*os.File, error) {
	return nil, errors.New("rooted audit directory open is unavailable on this platform")
}
