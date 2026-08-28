package governance

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// TerminalPrompt is intentionally line-oriented so it works with a controlling
// terminal and remains testable. EOF, an empty answer, and malformed answers
// all decline rather than granting an effect.
type TerminalPrompt struct {
	Input  io.Reader
	Output io.Writer
}

func (p TerminalPrompt) Decide(effect Effect) (Decision, error) {
	if p.Input == nil || p.Output == nil {
		return DecisionDecline, nil
	}
	if _, err := fmt.Fprintf(p.Output, "approve %s\n%s\n[y/N/c] ", TerminalSafe(effect.Kind), TerminalSafe(effect.ApprovalSummary())); err != nil {
		return DecisionDecline, err
	}
	s := bufio.NewScanner(p.Input)
	if !s.Scan() {
		if err := s.Err(); err != nil {
			return DecisionDecline, err
		}
		return DecisionDecline, nil
	}
	switch strings.ToLower(strings.TrimSpace(s.Text())) {
	case "y", "yes", "accept":
		return DecisionAccept, nil
	case "c", "cancel":
		return DecisionCancel, nil
	default:
		return DecisionDecline, nil
	}
}

const terminalFieldLimit = 4096

// TerminalSafe is the single rendering boundary for all untrusted terminal
// content. It preserves ordinary UTF-8/newlines and visibly escapes controls.
func TerminalSafe(value string) string {
	var b strings.Builder
	for _, r := range value {
		if b.Len() >= terminalFieldLimit {
			b.WriteString("…[truncated]")
			break
		}
		switch {
		case r == '\n' || r == '\t' || (r >= 0x20 && r != 0x7f && !(r >= 0x202a && r <= 0x202e) && !(r >= 0x2066 && r <= 0x2069)):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "\\u%04X", r)
		}
	}
	return b.String()
}
