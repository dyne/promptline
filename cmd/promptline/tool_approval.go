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

	"github.com/sashabaranov/go-openai"
	"golang.org/x/term"
	"promptline/internal/chat"
)

func newToolApprover() chat.ToolApprovalFunc {
	return func(call openai.ToolCall) (bool, error) {
		return promptToolApproval(call)
	}
}

func promptToolApproval(call openai.ToolCall) (bool, error) {
	input := os.Stdin
	output := io.Writer(os.Stdout)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
			input = tty
			output = tty
			defer tty.Close()
		} else {
			return false, fmt.Errorf("no TTY available for tool approval")
		}
	}
	reader := bufio.NewReader(input)
	name := call.Function.Name
	if name == "" {
		name = "unknown_tool"
	}
	rawArgs := strings.TrimSpace(call.Function.Arguments)
	argsDisplay := ""
	contentPreview := ""
	if rawArgs != "" && rawArgs != "{}" && rawArgs != "null" {
		argsDisplay = fmt.Sprintf(" with args %s", rawArgs)
		if argsMap, ok := parseArgsJSON(rawArgs); ok {
			if content, ok := argsMap["content"]; ok {
				if contentStr, ok := content.(string); ok {
					contentPreview = contentStr
				}
				delete(argsMap, "content")
			}
			if redacted, err := json.Marshal(argsMap); err == nil {
				argsDisplay = fmt.Sprintf(" with args %s", string(redacted))
			}
		}
	}

	for {
		fmt.Fprintf(output, "Allow tool %s%s? [y/N/p]: ", name, argsDisplay)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		normalized := strings.TrimSpace(strings.ToLower(input))
		switch normalized {
		case "y", "yes":
			return true, nil
		case "p":
			if contentPreview == "" {
				fmt.Fprintln(output, "No content field to preview.")
				continue
			}
			fmt.Fprintln(output, contentPreview)
			continue
		case "", "n", "no":
			return false, nil
		default:
			fmt.Fprintln(output, "Please enter y, n, or p to preview content.")
		}
	}
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
