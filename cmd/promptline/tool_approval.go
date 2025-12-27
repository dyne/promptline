// Copyright (C) 2025 Dyne.org foundation
// designed, written and maintained by Denis Roio <jaromil@dyne.org>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/term"
	"promptline/internal/chat"
)

type approvalDecision int

const (
	approvalUnknown approvalDecision = iota
	approvalYes
	approvalNo
	approvalAlways
)

type toolPromptFunc func(call openai.ToolCall) (approvalDecision, error)

func newToolApprover() chat.ToolApprovalFunc {
	return newToolApproverWithPrompt(promptToolApproval)
}

func newToolApproverWithPrompt(prompt toolPromptFunc) chat.ToolApprovalFunc {
	alwaysAllowed := make(map[string]bool)
	var mu sync.RWMutex
	return func(call openai.ToolCall) (bool, error) {
		toolName := toolCallName(call)
		mu.RLock()
		allowed := alwaysAllowed[toolName]
		mu.RUnlock()
		if allowed {
			return true, nil
		}

		decision, err := prompt(call)
		if err != nil {
			return false, err
		}
		if decision == approvalAlways {
			mu.Lock()
			alwaysAllowed[toolName] = true
			mu.Unlock()
			return true, nil
		}
		return decision == approvalYes, nil
	}
}

func promptToolApproval(call openai.ToolCall) (approvalDecision, error) {
	input := os.Stdin
	output := io.Writer(os.Stdout)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
			input = tty
			output = tty
			defer tty.Close()
		} else {
			return approvalNo, fmt.Errorf("no TTY available for tool approval")
		}
	}
	reader := bufio.NewReader(input)
	name := toolCallName(call)
	rawArgs := strings.TrimSpace(call.Function.Arguments)
	argsDisplay := ""
	if rawArgs != "" && rawArgs != "{}" && rawArgs != "null" {
		argsDisplay = fmt.Sprintf(" with args %s", rawArgs)
		if argsMap, ok := parseArgsJSON(rawArgs); ok {
			delete(argsMap, "content")
			if redacted, err := json.Marshal(argsMap); err == nil {
				argsDisplay = fmt.Sprintf(" with args %s", string(redacted))
			}
		}
	}

	for {
		fmt.Fprintf(output, "Allow tool %s%s? (Yes/no/always): ", name, argsDisplay)
		line, err := reader.ReadString('\n')
		if err != nil {
			return approvalNo, err
		}
		decision := parseApprovalInput(line)
		switch decision {
		case approvalYes, approvalNo, approvalAlways:
			return decision, nil
		default:
			fmt.Fprintln(output, "Please enter yes, no, or always.")
		}
	}
}

func parseApprovalInput(input string) approvalDecision {
	normalized := strings.TrimSpace(strings.ToLower(input))
	if normalized == "" {
		return approvalYes
	}
	switch {
	case isPrefixToken(normalized, "yes"):
		return approvalYes
	case isPrefixToken(normalized, "no"):
		return approvalNo
	case isPrefixToken(normalized, "always"):
		return approvalAlways
	default:
		return approvalUnknown
	}
}

func isPrefixToken(input, target string) bool {
	if input == "" || len(input) > len(target) {
		return false
	}
	return strings.HasPrefix(target, input)
}

func toolCallName(call openai.ToolCall) string {
	name := call.Function.Name
	if name == "" {
		return "unknown_tool"
	}
	return name
}

func parseArgsJSON(rawArgs string) (map[string]interface{}, bool) {
	if rawArgs == "" {
		return nil, false
	}
	var argsMap map[string]interface{}
	if err := json.Unmarshal([]byte(rawArgs), &argsMap); err != nil {
		return nil, false
	}
	return argsMap, true
}
