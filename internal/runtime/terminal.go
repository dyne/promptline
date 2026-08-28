package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"promptline/internal/governance"
)

// Terminal is intentionally line-oriented so tmux remains an external concern.
type Terminal struct{ Out io.Writer }

func (t Terminal) Prompt() error { _, err := fmt.Fprint(t.Out, "> "); return err }
func (t Terminal) Delta(s string) error {
	_, err := fmt.Fprint(t.Out, governance.TerminalSafe(s))
	return err
}
func (t Terminal) Text(s string) error {
	_, err := fmt.Fprintln(t.Out, governance.TerminalSafe(s))
	return err
}
func (t Terminal) Progress(s string) error {
	_, err := fmt.Fprintln(t.Out, "[", governance.TerminalSafe(s), "]")
	return err
}
func (t Terminal) Error(err error) error {
	_, writeErr := fmt.Fprintln(t.Out, "error:", governance.TerminalSafe(formatError(strings.TrimSpace(err.Error()))))
	return writeErr
}

func formatError(message string) string {
	detail, ok := decodeStructuredError(message)
	if !ok || detail.message == "" {
		return message
	}
	var context []string
	if detail.kind != "" && detail.kind != "error" {
		context = append(context, detail.kind)
	}
	if detail.status != "" {
		context = append(context, "HTTP "+detail.status)
	}
	if len(context) == 0 {
		return detail.message
	}
	return detail.message + " (" + strings.Join(context, ", ") + ")"
}

type structuredError struct {
	message string
	kind    string
	status  string
}

func decodeStructuredError(value string) (structuredError, bool) {
	var wire struct {
		Type    string          `json:"type"`
		Status  json.RawMessage `json:"status"`
		Message string          `json:"message"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal([]byte(value), &wire); err != nil {
		return structuredError{}, false
	}
	detail := structuredError{message: wire.Message, kind: wire.Type}
	if len(wire.Status) != 0 {
		var statusNumber json.Number
		if json.Unmarshal(wire.Status, &statusNumber) == nil {
			detail.status = statusNumber.String()
		} else {
			var statusString string
			if json.Unmarshal(wire.Status, &statusString) == nil {
				detail.status = statusString
			}
		}
	}
	if len(wire.Error) != 0 && string(wire.Error) != "null" {
		if nested, ok := decodeStructuredError(string(wire.Error)); ok {
			if nested.message != "" {
				detail.message = nested.message
			}
			if nested.kind != "" {
				detail.kind = nested.kind
			}
			if detail.status == "" {
				detail.status = nested.status
			}
		} else {
			var nestedMessage string
			if json.Unmarshal(wire.Error, &nestedMessage) == nil {
				detail.message = nestedMessage
			}
		}
	}
	if nested, ok := decodeStructuredError(detail.message); ok && nested.message != "" {
		if nested.kind == "" {
			nested.kind = detail.kind
		}
		if nested.status == "" {
			nested.status = detail.status
		}
		detail = nested
	}
	if detail.status != "" {
		if status, err := strconv.Atoi(detail.status); err == nil {
			detail.status = strconv.Itoa(status)
		}
	}
	return detail, detail.message != "" || detail.kind != "" || detail.status != ""
}
