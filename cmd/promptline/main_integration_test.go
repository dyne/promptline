//go:build integration && unix

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	mockCodexEnvironment   = "PROMPTLINE_MOCK_CODEX"
	mockAuthEnvironment    = "PROMPTLINE_MOCK_AUTHENTICATED"
	mockToolboxEnvironment = "PROMPTLINE_MOCK_TOOLBOX"
)

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func TestPromptlineRunsTurnWithMockCodex(t *testing.T) {
	work := t.TempDir()
	stateRoot := filepath.Join(t.TempDir(), "instances")
	mockCodex := writeMockCodexExecutable(t, true)
	inputReader, inputWriter := io.Pipe()
	t.Cleanup(func() { _ = inputWriter.Close() })

	var output lockedBuffer
	var stderr bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- run(
			[]string{
				"--instance", "integration",
				"--cwd", work,
				"--state-root", stateRoot,
				"--mock-codex", mockCodex,
			},
			inputReader,
			&output,
			&stderr,
		)
	}()

	if _, err := io.WriteString(inputWriter, "hello mock\n"); err != nil {
		t.Fatal(err)
	}
	waitForOutput(t, &output, "mock reply")
	if _, err := io.WriteString(inputWriter, "/quit\n"); err != nil {
		t.Fatal(err)
	}
	if err := inputWriter.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Promptline run failed: %v\nstderr: %s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Promptline did not stop after /quit")
	}
	if !strings.Contains(output.String(), "[ working ]") {
		t.Fatalf("working indicator was not rendered: %q", output.String())
	}
	if !strings.Contains(output.String(), "[ toolbox ready:") ||
		!strings.Contains(output.String(), "[ dynamicToolCall ]") {
		t.Fatalf("toolbox readiness and call progress were not rendered: %q", output.String())
	}
	if got := strings.Count(output.String(), "mock reply"); got != 1 {
		t.Fatalf("mock reply rendered %d times: %q", got, output.String())
	}

	state, err := os.ReadFile(filepath.Join(stateRoot, "integration", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(state, []byte(`"lastPrimaryThreadId":"thread-integration"`)) {
		t.Fatalf("primary thread was not persisted: %s", state)
	}
	if !bytes.Contains(state, []byte(`"codexVersion":"0.149.0"`)) {
		t.Fatalf("Codex CLI version was not persisted: %s", state)
	}
	config, err := os.ReadFile(filepath.Join(stateRoot, "integration", "codex-home", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(config, []byte("[mcp_servers.promptline-toolbox]")) {
		t.Fatalf("toolbox MCP configuration was not installed: %s", config)
	}
}

func TestVersionReportIncludesInstalledAndVendoredComponents(t *testing.T) {
	mockCodex := writeMockCodexExecutable(t, true)
	var output bytes.Buffer
	var stderr bytes.Buffer
	if err := run(
		[]string{"--version", "--mock-codex", mockCodex},
		nil,
		&output,
		&stderr,
	); err != nil {
		t.Fatalf("version report failed: %v\nstderr: %s", err, stderr.String())
	}
	for _, expected := range []string{
		"promptline: ",
		"codex-cli: 0.149.0",
		"u-root: v0.15.0",
		"go: go",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("version output is missing %q: %s", expected, output.String())
		}
	}
}

func TestPromptlineRejectsUnauthenticatedCodexBeforePrompt(t *testing.T) {
	work := t.TempDir()
	stateRoot := filepath.Join(t.TempDir(), "instances")
	mockCodex := writeMockCodexExecutable(t, false)
	var output bytes.Buffer
	var stderr bytes.Buffer
	err := run(
		[]string{
			"--instance", "unauthenticated",
			"--cwd", work,
			"--state-root", stateRoot,
			"--mock-codex", mockCodex,
		},
		strings.NewReader("ls\n"),
		&output,
		&stderr,
	)
	if err == nil {
		t.Fatal("unauthenticated Promptline startup succeeded")
	}
	for _, expected := range []string{
		"codex authentication required",
		"CODEX_HOME=",
		"login",
		"restart Promptline",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("authentication error is missing %q: %v", expected, err)
		}
	}
	if output.Len() != 0 {
		t.Fatalf("unauthenticated startup reached terminal prompt: %q", output.String())
	}
	if _, statErr := os.Stat(filepath.Join(stateRoot, "unauthenticated", "state.json")); !os.IsNotExist(statErr) {
		t.Fatalf("unauthenticated startup persisted thread state: %v", statErr)
	}
}

func TestPromptlineDefaultsToNewThreadAcrossEOFRestarts(t *testing.T) {
	work := t.TempDir()
	stateRoot := filepath.Join(t.TempDir(), "instances")
	mockCodex := writeMockCodexExecutable(t, true)
	for attempt := 1; attempt <= 2; attempt++ {
		var output bytes.Buffer
		var stderr bytes.Buffer
		err := run(
			[]string{
				"--instance", "restart-integration",
				"--cwd", work,
				"--state-root", stateRoot,
				"--mock-codex", mockCodex,
			},
			strings.NewReader(""),
			&output,
			&stderr,
		)
		if err != nil {
			t.Fatalf("attempt %d failed after EOF restart: %v\nstderr: %s", attempt, err, stderr.String())
		}
		if !strings.Contains(output.String(), "[ toolbox ready:") {
			t.Fatalf("attempt %d did not reach a ready prompt: %q", attempt, output.String())
		}
	}
}

func TestToolboxServesBasicURootTools(t *testing.T) {
	testStandaloneToolbox(t, []string{"mcp-server"})
}

func testStandaloneToolbox(t *testing.T, commandArguments []string) {
	t.Helper()
	work := t.TempDir()
	unusedStateRoot := filepath.Join(t.TempDir(), "must-not-be-created")
	fixturePath := filepath.Join(work, "fixture.txt")
	if err := os.WriteFile(fixturePath, []byte("fixture contents\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"pwd","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hello toolbox"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"cat","arguments":{"path":"fixture.txt"}}}`,
	}
	input := strings.NewReader(strings.Join(requests, "\n") + "\n")
	var output bytes.Buffer
	var stderr bytes.Buffer
	err := run(
		append(
			append([]string(nil), commandArguments...),
			"--cwd", work, "--state-root", unusedStateRoot,
		),
		input,
		&output,
		&stderr,
	)
	if err != nil {
		t.Fatalf("toolbox server failed: %v\nstderr: %s", err, stderr.String())
	}

	responses := decodeMCPResponses(t, output.Bytes())
	assertToolList(t, responses[2], "pwd", "echo", "cat")
	assertToolResultContains(t, responses[3], work)
	assertToolResultContains(t, responses[4], "hello toolbox")
	assertToolResultContains(t, responses[5], "fixture contents")
	if _, err := os.Stat(unusedStateRoot); !os.IsNotExist(err) {
		t.Fatalf("standalone MCP server created instance state: %v", err)
	}
}

func TestMockCodexProcess(t *testing.T) {
	if os.Getenv(mockCodexEnvironment) != "1" {
		return
	}
	if slicesContain(os.Args, "--version") {
		fmt.Println("codex-cli 0.149.0")
		os.Exit(0)
	}
	if !slicesContain(os.Args, "app-server") || !slicesContain(os.Args, "--stdio") {
		os.Exit(2)
	}
	serveMockCodex()
	os.Exit(0)
}

func TestMockToolboxProcess(t *testing.T) {
	if os.Getenv(mockToolboxEnvironment) != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		os.Exit(2)
	}
	if err := run(os.Args[separator+1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func writeMockCodexExecutable(t *testing.T, authenticated bool) string {
	t.Helper()
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "codex")
	authenticatedValue := "0"
	if authenticated {
		authenticatedValue = "1"
	}
	script := fmt.Sprintf(
		"#!/bin/sh\n%s=1 %s=%s exec %q -test.run '^TestMockCodexProcess$' -- \"$@\"\n",
		mockCodexEnvironment,
		mockAuthEnvironment,
		authenticatedValue,
		testExecutable,
	)
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func serveMockCodex() {
	const dynamicCallID uint64 = 9000
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	var toolbox *mockToolboxClient
	defer func() {
		if toolbox != nil {
			_ = toolbox.Close()
		}
	}()
	for scanner.Scan() {
		var request struct {
			ID     *uint64         `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == nil {
			continue
		}
		if request.Method == "" {
			if *request.ID == dynamicCallID {
				var reply struct {
					ContentItems []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"contentItems"`
					Success bool `json:"success"`
				}
				if json.Unmarshal(request.Result, &reply) != nil || !reply.Success || len(reply.ContentItems) != 1 || reply.ContentItems[0].Type != "inputText" {
					return
				}
				emitMockTurn(encoder, reply.ContentItems[0].Text, "dynamicToolCall")
			}
			continue
		}
		result := any(map[string]any{})
		switch request.Method {
		case "initialize":
			var params struct {
				Capabilities struct {
					Experimental bool `json:"experimentalApi"`
				} `json:"capabilities"`
			}
			if json.Unmarshal(request.Params, &params) != nil || !params.Capabilities.Experimental {
				_ = encoder.Encode(map[string]any{"id": *request.ID, "error": map[string]any{"code": -32602, "message": "dynamic tools require experimentalApi"}})
				continue
			}
		case "account/read":
			if os.Getenv(mockAuthEnvironment) == "1" {
				result = map[string]any{
					"account":            map[string]any{"type": "mock"},
					"requiresOpenaiAuth": true,
				}
			} else {
				result = map[string]any{"account": nil, "requiresOpenaiAuth": true}
			}
		case "thread/start":
			var params struct {
				DynamicTools []struct {
					Name  string `json:"name"`
					Tools []struct {
						Name string `json:"name"`
					} `json:"tools"`
				} `json:"dynamicTools"`
			}
			if json.Unmarshal(request.Params, &params) != nil || !hasDynamicTool(params.DynamicTools, "toolbox", "echo") {
				_ = encoder.Encode(map[string]any{"id": *request.ID, "error": map[string]any{"code": -32602, "message": "toolbox dynamic tools missing"}})
				continue
			}
			result = map[string]any{"thread": map[string]any{
				"id":     "thread-integration",
				"status": map[string]any{"type": "idle"},
			}}
		case "turn/start":
			result = map[string]any{"turn": map[string]any{
				"id":     "turn-integration",
				"status": "inProgress",
			}}
		case "mcpServerStatus/list":
			var err error
			toolbox, err = startConfiguredToolbox()
			if err != nil {
				_ = encoder.Encode(map[string]any{
					"id":    *request.ID,
					"error": map[string]any{"code": -32000, "message": err.Error()},
				})
				continue
			}
			result = map[string]any{
				"data": []map[string]any{{
					"name": "promptline-toolbox", "tools": toolbox.tools,
					"resources": []any{}, "resourceTemplates": []any{}, "authStatus": "unsupported",
				}},
				"nextCursor": nil,
			}
		case "mcpServer/tool/call":
			var params struct {
				Server    string         `json:"server"`
				Tool      string         `json:"tool"`
				Arguments map[string]any `json:"arguments"`
			}
			if json.Unmarshal(request.Params, &params) != nil || params.Server != "promptline-toolbox" || toolbox == nil {
				_ = encoder.Encode(map[string]any{"id": *request.ID, "error": map[string]any{"code": -32602, "message": "invalid MCP tool call"}})
				continue
			}
			text, err := toolbox.CallText(params.Tool, params.Arguments)
			if err != nil {
				_ = encoder.Encode(map[string]any{"id": *request.ID, "error": map[string]any{"code": -32000, "message": err.Error()}})
				continue
			}
			result = map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
		}
		_ = encoder.Encode(map[string]any{"id": *request.ID, "result": result})
		if request.Method == "turn/start" {
			_ = encoder.Encode(map[string]any{
				"id":     dynamicCallID,
				"method": "item/tool/call",
				"params": map[string]any{
					"threadId": "thread-integration", "turnId": "turn-integration", "callId": "call-integration",
					"namespace": "toolbox", "tool": "echo", "arguments": map[string]any{"text": "mock reply"},
				},
			})
		}
	}
}

func hasDynamicTool(namespaces []struct {
	Name  string `json:"name"`
	Tools []struct {
		Name string `json:"name"`
	} `json:"tools"`
}, namespaceName, toolName string) bool {
	for _, namespace := range namespaces {
		if namespace.Name != namespaceName {
			continue
		}
		for _, tool := range namespace.Tools {
			if tool.Name == toolName {
				return true
			}
		}
	}
	return false
}

func emitMockTurn(encoder *json.Encoder, reply, toolType string) {
	_ = encoder.Encode(map[string]any{"method": "item/started", "params": map[string]any{
		"threadId": "thread-integration", "turnId": "turn-integration",
		"item": map[string]any{"id": "tool-integration", "type": toolType},
	}})
	_ = encoder.Encode(map[string]any{"method": "item/completed", "params": map[string]any{
		"threadId": "thread-integration", "turnId": "turn-integration",
		"item": map[string]any{"id": "tool-integration", "type": toolType},
	}})
	_ = encoder.Encode(map[string]any{"method": "item/started", "params": map[string]any{
		"threadId": "thread-integration", "turnId": "turn-integration",
		"item": map[string]any{"id": "item-integration", "type": "agentMessage", "text": ""},
	}})
	_ = encoder.Encode(map[string]any{"method": "item/agentMessage/delta", "params": map[string]any{
		"threadId": "thread-integration", "turnId": "turn-integration", "itemId": "item-integration",
		"delta": strings.TrimSuffix(reply, "reply"),
	}})
	_ = encoder.Encode(map[string]any{"method": "item/agentMessage/delta", "params": map[string]any{
		"threadId": "thread-integration", "turnId": "turn-integration", "itemId": "item-integration",
		"delta": strings.TrimPrefix(reply, "mock "),
	}})
	_ = encoder.Encode(map[string]any{"method": "item/completed", "params": map[string]any{
		"threadId": "thread-integration", "turnId": "turn-integration",
		"item": map[string]any{"id": "item-integration", "type": "agentMessage", "text": reply},
	}})
	_ = encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"turn": map[string]any{
		"id": "turn-integration", "status": "completed",
		"items": []map[string]any{{"id": "item-integration", "type": "agentMessage", "text": reply}},
	}}})
}

type mockToolboxClient struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Scanner
	nextID  int
	tools   map[string]json.RawMessage
	stderr  bytes.Buffer
}

func startConfiguredToolbox() (*mockToolboxClient, error) {
	command, arguments, err := readConfiguredToolbox(filepath.Join(os.Getenv("CODEX_HOME"), "config.toml"))
	if err != nil {
		return nil, err
	}
	testExecutable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if command == testExecutable {
		arguments = append([]string{"-test.run", "^TestMockToolboxProcess$", "--"}, arguments...)
	}
	process := exec.Command(command, arguments...)
	process.Env = append(os.Environ(), mockToolboxEnvironment+"=1")
	stdin, err := process.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		return nil, err
	}
	client := &mockToolboxClient{command: process, stdin: stdin, stdout: bufio.NewScanner(stdout)}
	process.Stderr = &client.stderr
	if err := process.Start(); err != nil {
		return nil, err
	}
	if _, err := client.call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo":      map[string]string{"name": "mock-codex", "version": "test"},
		"capabilities":    map[string]any{},
	}); err != nil {
		_ = client.Close()
		return nil, err
	}
	result, err := client.call("tools/list", map[string]any{})
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	var list struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(result, &list); err != nil {
		_ = client.Close()
		return nil, err
	}
	client.tools = make(map[string]json.RawMessage, len(list.Tools))
	for _, definition := range list.Tools {
		var identity struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(definition, &identity) == nil && identity.Name != "" {
			client.tools[identity.Name] = definition
		}
	}
	for _, required := range []string{"ls", "pwd", "cat", "echo"} {
		if _, ok := client.tools[required]; !ok {
			_ = client.Close()
			return nil, fmt.Errorf("configured toolbox is missing %q", required)
		}
	}
	if result, err := client.CallText("pwd", map[string]any{}); err != nil || strings.TrimSpace(result) == "" {
		_ = client.Close()
		return nil, fmt.Errorf("call toolbox pwd: result=%q error=%w", result, err)
	}
	return client, nil
}

func readConfiguredToolbox(path string) (string, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var command string
	var arguments []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "command = "):
			command, err = strconv.Unquote(strings.TrimSpace(strings.TrimPrefix(line, "command = ")))
		case strings.HasPrefix(line, "args = "):
			err = json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "args = "))), &arguments)
		}
		if err != nil {
			return "", nil, err
		}
	}
	if command == "" || len(arguments) == 0 {
		return "", nil, errors.New("promptline-toolbox command is missing from Codex config")
	}
	return command, arguments, nil
}

func (c *mockToolboxClient) call(method string, params any) (json.RawMessage, error) {
	c.nextID++
	request := map[string]any{
		"jsonrpc": "2.0", "id": c.nextID, "method": method, "params": params,
	}
	if err := json.NewEncoder(c.stdin).Encode(request); err != nil {
		return nil, err
	}
	if !c.stdout.Scan() {
		return nil, fmt.Errorf("toolbox exited before %s response: %s", method, c.stderr.String())
	}
	var response struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(c.stdout.Bytes(), &response); err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, errors.New(response.Error.Message)
	}
	return response.Result, nil
}

func (c *mockToolboxClient) CallText(name string, arguments map[string]any) (string, error) {
	result, err := c.call("tools/call", map[string]any{"name": name, "arguments": arguments})
	if err != nil {
		return "", err
	}
	var decoded struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		return "", err
	}
	if decoded.IsError || len(decoded.Content) == 0 {
		return "", fmt.Errorf("tool %s failed: %s", name, result)
	}
	return decoded.Content[0].Text, nil
}

func (c *mockToolboxClient) Close() error {
	_ = c.stdin.Close()
	return c.command.Wait()
}

func waitForOutput(t *testing.T, output *lockedBuffer, text string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatalf("output did not contain %q: %s", text, output.String())
		case <-ticker.C:
			if strings.Contains(output.String(), text) {
				return
			}
		}
	}
}

func slicesContain(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type mcpResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
}

func decodeMCPResponses(t *testing.T, output []byte) map[int]mcpResponse {
	t.Helper()
	responses := map[int]mcpResponse{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var response mcpResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		responses[response.ID] = response
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(responses) != 5 {
		t.Fatalf("MCP responses = %d, want 5: %s", len(responses), output)
	}
	return responses
}

func assertToolList(t *testing.T, response mcpResponse, expected ...string) {
	t.Helper()
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	available := make(map[string]struct{}, len(result.Tools))
	for _, tool := range result.Tools {
		available[tool.Name] = struct{}{}
	}
	for _, name := range expected {
		if _, ok := available[name]; !ok {
			t.Errorf("tools/list is missing %q", name)
		}
	}
}

func assertToolResultContains(t *testing.T, response mcpResponse, expected string) {
	t.Helper()
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("tool call failed: %s", response.Result)
	}
	if !strings.Contains(result.Content[0].Text, expected) {
		t.Fatalf("tool output %q does not contain %q", result.Content[0].Text, expected)
	}
}
