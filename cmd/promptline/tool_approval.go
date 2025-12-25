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
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"promptline/internal/chat"
	"promptline/internal/theme"
)

func newToolApprover(colors *theme.ColorScheme) chat.ToolApprovalFunc {
	return func(call openai.ToolCall) (bool, error) {
		return promptToolApproval(call, colors)
	}
}

func promptToolApproval(call openai.ToolCall, colors *theme.ColorScheme) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	name := call.Function.Name
	if name == "" {
		name = "unknown_tool"
	}
	args := strings.TrimSpace(call.Function.Arguments)
	argsDisplay := ""
	if args != "" && args != "{}" && args != "null" {
		argsDisplay = fmt.Sprintf(" with args %s", args)
	}

	for {
		if colors != nil {
			colors.Header.Print("Permission: ")
		}
		fmt.Printf("Allow tool %s%s? [y/N]: ", name, argsDisplay)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		normalized := strings.TrimSpace(strings.ToLower(input))
		switch normalized {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			return false, nil
		default:
			fmt.Println("Please enter y or n.")
		}
	}
}
