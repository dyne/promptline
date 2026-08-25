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
	"bytes"
	"io"
	"strings"
	"testing"

	"promptline/internal/governance"
	"promptline/internal/instance"
)

func TestVersionVariable(t *testing.T) {
	// Test that Version variable exists and has default value
	if Version == "" {
		t.Error("Version variable should not be empty")
	}

	// Default value should be "dev" if not set via ldflags
	if Version != "dev" {
		t.Logf("Note: Version is set to %q (may be set via ldflags)", Version)
	}
}

func TestExitCodeReportsFatalErrors(t *testing.T) {
	var stderr bytes.Buffer
	if code := exitCode([]string{"--unknown"}, nil, io.Discard, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("promptline:")) {
		t.Fatalf("missing fatal error prefix: %q", stderr.String())
	}
}

func TestExitCodeReturnsSuccessForVersion(t *testing.T) {
	var output bytes.Buffer
	if code := exitCode([]string{"--version"}, nil, &output, io.Discard); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, component := range []string{"promptline:", "codex-cli:", "u-root:", "go:"} {
		if !strings.Contains(output.String(), component) {
			t.Fatalf("missing %s version output: %q", component, output.String())
		}
	}
}

func TestExitCodeReturnsSuccessForHelp(t *testing.T) {
	var stderr bytes.Buffer
	if code := exitCode([]string{"--help"}, nil, io.Discard, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Usage: promptline [OPTIONS]")) ||
		!bytes.Contains(stderr.Bytes(), []byte("mcp-server")) {
		t.Fatalf("missing help output: %q", stderr.String())
	}
	if bytes.Contains(stderr.Bytes(), []byte("promptline: flag: help requested")) {
		t.Fatalf("help was reported as fatal: %q", stderr.String())
	}
}

func TestApprovalPromptHonorsMode(t *testing.T) {
	if prompt := approvalPrompt(instance.ApprovalDeny, nil, io.Discard); prompt != nil {
		t.Fatal("deny mode created an interactive prompt")
	}
	prompt := approvalPrompt(instance.ApprovalAsk, nil, io.Discard)
	if _, ok := prompt.(governance.TerminalPrompt); !ok {
		t.Fatalf("ask mode prompt = %T, want TerminalPrompt", prompt)
	}
}
