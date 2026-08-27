package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"promptline/internal/instance"
	"promptline/internal/tools"
	"promptline/plugins/promptline/skills"
)

func TestServerLifecycleAndDeniedCall(t *testing.T) {
	config := tools.DefaultConfig()
	config.WorkingDirectory = t.TempDir()
	registry, err := tools.NewRegistryWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })
	if err := registry.RegisterTool(&tools.ToolDefinition{NameValue: "test_echo", DescriptionValue: "echo", ParametersValue: map[string]interface{}{"type": "object"}, ExecuteFunc: func(context.Context, map[string]interface{}) (string, error) { return "ok", nil }}); err != nil {
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

func TestServerListsAndReadsEmbeddedResources(t *testing.T) {
	catalog, err := skills.EmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	registry := tools.NewRegistry()
	t.Cleanup(func() { _ = registry.Close() })
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"skill://debian-sysadmin/SKILL.md"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"skill://debian-sysadmin/references/systemd.md"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"skill://debian-sysadmin/playbooks/disk-full.md"}}`,
	}
	var output bytes.Buffer
	server, err := NewServer(registry, strings.NewReader(strings.Join(requests, "\n")+"\n"), &output, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(t.Context()); err != nil {
		t.Fatal(err)
	}
	responses := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(responses) != len(requests) {
		t.Fatalf("responses = %d, want %d", len(responses), len(requests))
	}

	var initialized struct {
		Result struct {
			Capabilities map[string]json.RawMessage `json:"capabilities"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(responses[0]), &initialized); err != nil {
		t.Fatal(err)
	}
	if _, ok := initialized.Result.Capabilities["tools"]; !ok {
		t.Fatal("initialize omitted tools capability")
	}
	if _, ok := initialized.Result.Capabilities["resources"]; !ok {
		t.Fatal("initialize omitted resources capability")
	}

	var listed struct {
		Result struct {
			Resources []struct {
				URI      string `json:"uri"`
				Name     string `json:"name"`
				Title    string `json:"title"`
				MIMEType string `json:"mimeType"`
			} `json:"resources"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(responses[2]), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Result.Resources) != 33 {
		t.Fatalf("resources/list count = %d, want 33", len(listed.Result.Resources))
	}
	wantURIs := []string{
		"skill://debian-sysadmin/CHANGELOG.md",
		"skill://debian-sysadmin/LICENSE",
		"skill://debian-sysadmin/README.md",
		"skill://debian-sysadmin/SKILL.md",
		"skill://debian-sysadmin/agents/openai.yaml",
		"skill://debian-sysadmin/docs/DESIGN.md",
		"skill://debian-sysadmin/docs/PROVENANCE.md",
		"skill://debian-sysadmin/docs/UPSTREAM-REVIEW.md",
		"skill://debian-sysadmin/playbooks/disk-full.md",
		"skill://debian-sysadmin/playbooks/dns-failure.md",
		"skill://debian-sysadmin/playbooks/failed-boot.md",
		"skill://debian-sysadmin/playbooks/failed-upgrade.md",
		"skill://debian-sysadmin/playbooks/high-load.md",
		"skill://debian-sysadmin/playbooks/networking-failure.md",
		"skill://debian-sysadmin/playbooks/package-failure.md",
		"skill://debian-sysadmin/playbooks/service-failure.md",
		"skill://debian-sysadmin/playbooks/ssh-failure.md",
		"skill://debian-sysadmin/references/apt-dpkg.md",
		"skill://debian-sysadmin/references/backups.md",
		"skill://debian-sysadmin/references/boot-recovery.md",
		"skill://debian-sysadmin/references/diagnostics.md",
		"skill://debian-sysadmin/references/dns.md",
		"skill://debian-sysadmin/references/networking.md",
		"skill://debian-sysadmin/references/nftables.md",
		"skill://debian-sysadmin/references/performance.md",
		"skill://debian-sysadmin/references/principles.md",
		"skill://debian-sysadmin/references/security.md",
		"skill://debian-sysadmin/references/shell-safety.md",
		"skill://debian-sysadmin/references/ssh.md",
		"skill://debian-sysadmin/references/storage.md",
		"skill://debian-sysadmin/references/systemd.md",
		"skill://debian-sysadmin/references/toolbox.md",
		"skill://debian-sysadmin/references/users-permissions.md",
	}
	gotURIs := make([]string, 0, len(listed.Result.Resources))
	previous := ""
	for _, resource := range listed.Result.Resources {
		if resource.URI <= previous || resource.Name == "" || resource.Title == "" || resource.MIMEType == "" {
			t.Fatalf("invalid or unsorted resource: %+v", resource)
		}
		if strings.Contains(resource.URI, "/scripts/") || strings.Contains(resource.URI, "/tests/") {
			t.Fatalf("excluded resource listed: %q", resource.URI)
		}
		previous = resource.URI
		gotURIs = append(gotURIs, resource.URI)
	}
	if !slices.Equal(gotURIs, wantURIs) {
		t.Fatalf("resources/list URIs = %q, want %q", gotURIs, wantURIs)
	}

	for index, file := range []string{"SKILL.md", "references/systemd.md", "playbooks/disk-full.md"} {
		var read struct {
			Result struct {
				Contents []struct {
					URI  string `json:"uri"`
					Text string `json:"text"`
				} `json:"contents"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(responses[index+3]), &read); err != nil {
			t.Fatal(err)
		}
		want, err := catalog.ReadFile("debian-sysadmin", file)
		if err != nil {
			t.Fatal(err)
		}
		if len(read.Result.Contents) != 1 || read.Result.Contents[0].Text != string(want) || read.Result.Contents[0].URI != "skill://debian-sysadmin/"+file {
			t.Fatalf("resources/read %q = %+v, want exact embedded content", file, read.Result.Contents)
		}
	}
}

func TestServerRejectsInvalidResourceRequests(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		params  string
		message string
	}{
		{name: "list cursor", method: "resources/list", params: `{"cursor":"next"}`, message: "cursors are not supported"},
		{name: "list malformed", method: "resources/list", params: `[]`, message: "invalid resources/list parameters"},
		{name: "read missing URI", method: "resources/read", params: `{}`, message: "invalid resources/read parameters"},
		{name: "read malformed", method: "resources/read", params: `[]`, message: "invalid resources/read parameters"},
		{name: "traversal", method: "resources/read", params: `{"uri":"skill://debian-sysadmin/../SKILL.md"}`, message: "invalid embedded skill resource URI"},
		{name: "absolute", method: "resources/read", params: `{"uri":"skill://debian-sysadmin//etc/passwd"}`, message: "invalid embedded skill resource URI"},
		{name: "noncanonical", method: "resources/read", params: `{"uri":"skill://debian-sysadmin/references%2Fsystemd.md"}`, message: "invalid embedded skill resource URI"},
		{name: "unknown skill", method: "resources/read", params: `{"uri":"skill://missing/SKILL.md"}`, message: "invalid embedded skill resource URI"},
		{name: "unknown file", method: "resources/read", params: `{"uri":"skill://debian-sysadmin/missing.md"}`, message: "invalid embedded skill resource URI"},
		{name: "excluded", method: "resources/read", params: `{"uri":"skill://debian-sysadmin/scripts/validate-skill.sh"}`, message: "invalid embedded skill resource URI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tools.NewRegistry()
			t.Cleanup(func() { _ = registry.Close() })
			input := `{"jsonrpc":"2.0","id":1,"method":"` + tt.method + `","params":` + tt.params + "}\n"
			var output bytes.Buffer
			server, err := NewServer(registry, strings.NewReader(input), &output, 4096)
			if err != nil {
				t.Fatal(err)
			}
			if err := server.Serve(t.Context()); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), `"code":-32602`) || !strings.Contains(output.String(), tt.message) || strings.Contains(output.String(), "/home/") {
				t.Fatalf("response = %s", output.String())
			}
		})
	}
}

func TestServerRejectsResourceResponseExceedingFrameLimit(t *testing.T) {
	catalog, err := skills.NewCatalog(fstest.MapFS{
		"example/SKILL.md": &fstest.MapFile{Data: []byte(strings.Repeat("x", 4096))},
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := tools.NewRegistry()
	t.Cleanup(func() { _ = registry.Close() })
	var output bytes.Buffer
	server, err := NewServerWithCatalog(registry, catalog, strings.NewReader(`{"id":1,"method":"resources/read","params":{"uri":"skill://example/SKILL.md"}}`+"\n"), &output, 512)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"code":-32000`) || !strings.Contains(output.String(), "resource response exceeds frame limit") {
		t.Fatalf("response = %s", output.String())
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
		!strings.Contains(config, `"mcp-server", "--cwd"`) {
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

func TestDynamicToolboxExposesRegisteredToolsDeterministically(t *testing.T) {
	config := tools.DefaultConfig()
	config.WorkingDirectory = t.TempDir()
	registry, err := tools.NewRegistryWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })
	namespace := DynamicToolbox(registry)
	if namespace.Type != "namespace" || namespace.Name != "toolbox" {
		t.Fatalf("namespace = %+v", namespace)
	}
	if len(namespace.Tools) != len(registry.GetTools()) {
		t.Fatalf("dynamic tools = %d, registered = %d", len(namespace.Tools), len(registry.GetTools()))
	}
	foundPWD := false
	for index, tool := range namespace.Tools {
		if index > 0 && namespace.Tools[index-1].Name > tool.Name {
			t.Fatalf("tools are not sorted at %q", tool.Name)
		}
		if tool.Name == "pwd" {
			foundPWD = tool.Type == "function" && tool.InputSchema["type"] == "object"
		}
	}
	if !foundPWD {
		t.Fatal("model-facing toolbox is missing a valid pwd definition")
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
