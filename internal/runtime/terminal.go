package runtime

import (
	"fmt"
	"io"
	"strings"
)

// Terminal is intentionally line-oriented so tmux remains an external concern.
type Terminal struct{ Out io.Writer }

func (t Terminal) Prompt() error           { _, err := fmt.Fprint(t.Out, "> "); return err }
func (t Terminal) Text(s string) error     { _, err := fmt.Fprintln(t.Out, s); return err }
func (t Terminal) Progress(s string) error { _, err := fmt.Fprintln(t.Out, "[", s, "]"); return err }
func (t Terminal) Error(err error) error {
	_, writeErr := fmt.Fprintln(t.Out, "error:", strings.TrimSpace(err.Error()))
	return writeErr
}
