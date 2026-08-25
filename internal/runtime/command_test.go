package runtime

import (
	"io"
	"testing"

	"promptline/internal/instance"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"valid", []string{"--instance", "ops", "--cwd", "."}, false},
		{"unknown flag", []string{"--nope"}, true},
		{"missing cwd", []string{"--instance", "ops"}, true},
		{"version has no side effects", []string{"--version"}, false},
		{"version alias", []string{"-V"}, false},
		{"new command", []string{"new", "--cwd", "."}, false},
		{"resume last command", []string{"resume", "--cwd", "."}, false},
		{"resume ID command", []string{"resume", "thread-1", "--cwd", "."}, false},
		{"MCP server command", []string{"mcp-server"}, false},
		{"removed new flag", []string{"--cwd", ".", "--new"}, true},
		{"removed resume flag", []string{"--cwd", ".", "--resume", "x"}, true},
		{"removed MCP server flag", []string{"--mcp-server"}, true},
		{"removed toolbox serve command", []string{"toolbox", "serve", "--cwd", "."}, true},
		{"unknown command", []string{"chat", "--cwd", "."}, true},
		{"mock codex", []string{"--cwd", ".", "--mock-codex", "/tmp/mock-codex"}, false},
		{"conflicting codex executables", []string{"--cwd", ".", "--codex", "/tmp/codex", "--mock-codex", "/tmp/mock"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.args, io.Discard)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestParseThreadCommands(t *testing.T) {
	command, err := Parse([]string{"--cwd", "."}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !command.New || command.ResumeID != "" {
		t.Fatalf("default command = %+v, want new thread", command)
	}
	command, err = Parse([]string{"resume", "thread-1", "-C", ".", "-m", "custom"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if command.New || command.ResumeID != "thread-1" || command.Instance.Model != "custom" {
		t.Fatalf("resume command = %+v", command)
	}
	command, err = Parse([]string{"resume", "--cwd", "."}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if command.New || command.ResumeID != "" {
		t.Fatalf("resume-last command = %+v", command)
	}
}

func TestParseStandaloneMCPServer(t *testing.T) {
	command, err := Parse([]string{"mcp-server"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !command.ToolboxServe || command.Instance.WorkingDirectory == "" {
		t.Fatalf("Parse(mcp-server) = %+v", command)
	}
}

func TestParseMockCodex(t *testing.T) {
	command, err := Parse([]string{"--cwd", ".", "--mock-codex", "/tmp/mock-codex"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !command.MockCodex || command.Instance.CodexExecutable != "/tmp/mock-codex" {
		t.Fatalf("mock command = %+v", command)
	}
}

func TestParseToolboxDefaultsOnAndCanBeDisabled(t *testing.T) {
	command, err := Parse([]string{"--cwd", "."}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !command.Instance.ToolboxEnabled {
		t.Fatal("toolbox should be enabled by default")
	}
	command, err = Parse([]string{"--cwd", ".", "--toolbox=false"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if command.Instance.ToolboxEnabled {
		t.Fatal("--toolbox=false did not disable toolbox")
	}
}

func TestParseApprovalMode(t *testing.T) {
	command, err := Parse([]string{"--cwd", ".", "--approval", "ask"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if command.Instance.ApprovalMode != instance.ApprovalAsk {
		t.Fatalf("approval mode = %q, want ask", command.Instance.ApprovalMode)
	}
	if _, err := Parse([]string{"--cwd", ".", "--approval", "always"}, io.Discard); err == nil {
		t.Fatal("invalid approval mode accepted")
	}
}
