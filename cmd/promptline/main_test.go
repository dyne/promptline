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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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

func TestSkillCommandsUseEmbeddedCatalogWithoutInstanceState(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state-must-not-exist")
	var output bytes.Buffer
	if err := run([]string{"list-skills", "--state-root", stateRoot, "--mock-codex", "/must-not-run"}, nil, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	wantSkills := "bash-defensive-patterns\nbash-linux\ndebian-sysadmin\nsecurity-ownership-map\nsecurity-threat-model\n"
	if got := output.String(); got != wantSkills {
		t.Fatalf("list-skills output = %q, want %q", got, wantSkills)
	}
	output.Reset()
	if err := run([]string{"list-skill-files", "debian-sysadmin", "--state-root", stateRoot, "--mock-codex", "/must-not-run"}, nil, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(strings.TrimSpace(output.String()), "\n") + 1; got != 34 {
		t.Fatalf("list-skill-files count = %d, want 34", got)
	}
	output.Reset()
	if err := run([]string{"list-skill-files", "security-ownership-map", "--state-root", stateRoot, "--mock-codex", "/must-not-run"}, nil, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "scripts/query_ownership.py\n") {
		t.Fatalf("operational skill scripts are not discoverable: %q", output.String())
	}
	if _, err := os.Stat(stateRoot); !os.IsNotExist(err) {
		t.Fatalf("skill discovery created instance state: %v", err)
	}
}

func TestMaterializeSkillCommandDoesNotCreateInstanceState(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state-must-not-exist")
	destination := filepath.Join(t.TempDir(), "skills")
	if err := run([]string{"materialize-skill", destination, "--state-root", stateRoot, "--mock-codex", "/must-not-run"}, nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "debian-sysadmin", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "LICENSE.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stateRoot); !os.IsNotExist(err) {
		t.Fatalf("materialization created instance state: %v", err)
	}
}

func TestVerifyAuditCommandReportsLocalAndAnchoredEvidence(t *testing.T) {
	stateDir := t.TempDir()
	dir := filepath.Join(stateDir, "audit")
	j, err := governance.OpenJournal(governance.JournalConfig{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Append(governance.Event{Instance: "test", Kind: "effect"}, true); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "events.jsonl")
	var output bytes.Buffer
	if err := run([]string{"verify-audit", stateDir}, nil, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "local chain only") {
		t.Fatalf("local trust status = %q", output.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record governance.Event
	if err := json.Unmarshal(bytes.TrimSpace(data), &record); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := run([]string{"verify-audit", stateDir, "--audit-anchor", record.Hash}, nil, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "external anchor matches") {
		t.Fatalf("anchor status = %q", output.String())
	}
	if err := run([]string{"verify-audit", stateDir, "--audit-anchor", "wrong"}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("wrong anchor accepted")
	}
	if err := os.WriteFile(path, bytes.Replace(data, []byte(`"kind":"effect"`), []byte(`"kind":"alter"`), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"verify-audit", stateDir}, nil, io.Discard, io.Discard); err == nil {
		t.Fatal("tampered journal accepted")
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
