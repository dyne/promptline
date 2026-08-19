package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"promptline/internal/instance"
	"promptline/internal/tools"
)

func TestServerLifecycleAndDeniedCall(t *testing.T) {
	registry, err := tools.NewRegistryWithConfig(tools.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })
	if err := registry.RegisterTool(&tools.ToolDefinition{NameValue: "test_echo", DescriptionValue: "echo", ParametersValue: map[string]interface{}{"type": "object"}, VersionValue: tools.HostAPIVersion, ExecuteFunc: func(context.Context, map[string]interface{}) (string, error) { return "ok", nil }}); err != nil {
		t.Fatal(err)
	}
	input := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\",\"params\":{}}\n{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"test_echo\",\"arguments\":{}}}\n")
	var output bytes.Buffer
	server, err := NewServer(registry, input, &output, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("responses = %d, want 3", len(lines))
	}
	var list struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Result.Tools) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	var call struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &call); err != nil {
		t.Fatal(err)
	}
	if !call.Result.IsError {
		t.Fatal("default ask policy must deny noninteractive MCP call")
	}
}

func TestCodexConfigIsInstanceScoped(t *testing.T) {
	root := t.TempDir()
	in, err := instance.New(instance.Config{Name: "mcp-test", StateRoot: root, WorkingRoot: root, WorkingDirectory: root})
	if err != nil {
		t.Fatal(err)
	}
	data, err := CodexConfig("/usr/local/bin/promptline", in)
	if err != nil {
		t.Fatal(err)
	}
	config := string(data)
	if !strings.Contains(config, "[mcp_servers.promptline-toolbox]") ||
		!strings.Contains(config, `command = "/usr/local/bin/promptline"`) ||
		!strings.Contains(config, `"--instance", "mcp-test"`) {
		t.Fatalf("unexpected config: %s", data)
	}
	if strings.HasPrefix(strings.TrimSpace(config), "{") {
		t.Fatalf("Codex config must be TOML, got JSON: %s", data)
	}
	if _, err := CodexConfig("promptline", in); err == nil {
		t.Fatal("relative executable accepted")
	}
	if err := InstallCodexConfig("/usr/local/bin/promptline", in); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(in.CodexHome(), "config.toml")
	installed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != config {
		t.Fatalf("installed config differs:\n%s", installed)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o, want 600", info.Mode().Perm())
	}
}

func TestReadOnlyToolPolicyAllowsInspectionAndRequiresApprovalForMutation(t *testing.T) {
	root := t.TempDir()
	config := tools.DefaultConfig()
	config.WorkingDirectory = root
	config.Roots = []string{root}
	config.Policy = ReadOnlyToolPolicy()
	registry, err := tools.NewRegistryWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })

	if result := registry.ExecuteContext(t.Context(), "pwd", map[string]any{}); result.Error != nil {
		t.Fatalf("read-only tool failed: %v", result.Error)
	}
	if result := registry.ExecuteContext(t.Context(), "mkdir", map[string]any{"path": "created"}); result.Error == nil {
		t.Fatal("mutating tool ran without approval")
	}
}

func TestServerRejectsInvalidCall(t *testing.T) {
	registry := tools.NewRegistry()
	t.Cleanup(func() { _ = registry.Close() })
	var output bytes.Buffer
	server, err := NewServer(registry, strings.NewReader("{\"id\":1,\"method\":\"tools/call\",\"params\":{}}\n"), &output, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "-32602") {
		t.Fatalf("unexpected response: %s", output.String())
	}
}

func TestServerIncludesToolErrorText(t *testing.T) {
	registry := tools.NewRegistry()
	t.Cleanup(func() { _ = registry.Close() })
	var output bytes.Buffer
	server, err := NewServer(registry, strings.NewReader("{\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"missing\",\"arguments\":{}}}\n"), &output, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Result.IsError || len(response.Result.Content) != 1 || response.Result.Content[0].Text == "" {
		t.Fatalf("missing tool error detail: %s", output.String())
	}
}
