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
	if _, err := fmt.Fprintf(p.Output, "approve %s %s in %s? [y/N/c] ", effect.Kind, effect.Operation, effect.CWD); err != nil {
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
