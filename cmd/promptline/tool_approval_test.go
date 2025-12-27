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
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestParseApprovalInput(t *testing.T) {
	cases := []struct {
		input    string
		expected approvalDecision
	}{
		{"", approvalYes},
		{" ", approvalYes},
		{"Y", approvalYes},
		{"ye", approvalYes},
		{"yes", approvalYes},
		{"n", approvalNo},
		{"no", approvalNo},
		{"a", approvalAlways},
		{"al", approvalAlways},
		{"alw", approvalAlways},
		{"alwa", approvalAlways},
		{"alway", approvalAlways},
		{"always", approvalAlways},
		{"maybe", approvalUnknown},
		{"yess", approvalUnknown},
		{"nope", approvalUnknown},
		{"alwayz", approvalUnknown},
	}

	for _, tc := range cases {
		decision := parseApprovalInput(tc.input)
		if decision != tc.expected {
			t.Fatalf("input %q expected %v, got %v", tc.input, tc.expected, decision)
		}
	}
}

func TestToolApproverAlwaysPersists(t *testing.T) {
	prompts := 0
	approver := newToolApproverWithPrompt(func(call openai.ToolCall) (approvalDecision, error) {
		prompts++
		if call.Function.Name == "read_file" {
			return approvalAlways, nil
		}
		return approvalNo, nil
	})

	readCall := openai.ToolCall{
		Function: openai.FunctionCall{
			Name: "read_file",
		},
	}
	approved, err := approver(readCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected first read_file approval")
	}

	approved, err = approver(readCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected read_file to be auto-approved")
	}
	if prompts != 1 {
		t.Fatalf("expected prompt once, got %d", prompts)
	}

	editCall := openai.ToolCall{
		Function: openai.FunctionCall{
			Name: "edit_file",
		},
	}
	approved, err = approver(editCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Fatal("expected edit_file to remain denied")
	}
	if prompts != 2 {
		t.Fatalf("expected prompt count 2, got %d", prompts)
	}
}
