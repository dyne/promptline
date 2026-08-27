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
	"testing"
	"time"

	pruntime "promptline/internal/runtime"
	"promptline/internal/testsupport"
)

const (
	mockCodexEnvironment   = "PROMPTLINE_MOCK_CODEX"
	mockAuthEnvironment    = "PROMPTLINE_MOCK_AUTHENTICATED"
	mockToolboxEnvironment = "PROMPTLINE_MOCK_TOOLBOX"
)

func TestPromptlineRunsTurnWithMockCodex(t *testing.T) {
	work := t.TempDir()
	stateRoot := filepath.Join(t.TempDir(), "instances")
	mockCodex := writeMockCodexExecutable(t, true)
	inputReader, inputWriter := io.Pipe()
	t.Cleanup(func() { _ = inputWriter.Close() })

	var output testsupport.LockedBuffer
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
	if !errors.Is(err, pruntime.ErrAuthenticationRequired) {
		t.Fatalf("unauthenticated startup error = %v", err)
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
		`{"jsonrpc":"2.0","id":6,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":7,"method":"resources/read","params":{"uri":"skill://debian-sysadmin/SKILL.md"}}`,
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

	responses := testsupport.DecodeMCPResponses(t, output.Bytes())
	if len(responses) != 7 {
		t.Fatalf("MCP response count = %d, want 7", len(responses))
	}
	testsupport.AssertMCPToolList(t, responses[2], "pwd", "echo", "cat")
	testsupport.AssertMCPTextResult(t, responses[3], work)
	testsupport.AssertMCPTextResult(t, responses[4], "hello toolbox")
	testsupport.AssertMCPTextResult(t, responses[5], "fixture contents")
	var resources struct {
		Resources []struct {
			URI string `json:"uri"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(responses[6].Result, &resources); err != nil {
		t.Fatal(err)
	}
	foundSkill := false
	for _, resource := range resources.Resources {
		foundSkill = foundSkill || resource.URI == "skill://debian-sysadmin/SKILL.md"
	}
	if !foundSkill {
		t.Fatal("standalone toolbox omitted embedded skill resource")
	}
	var resource struct {
		Contents []struct {
			Text string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(responses[7].Result, &resource); err != nil {
		t.Fatal(err)
	}
	if len(resource.Contents) != 1 || !strings.Contains(resource.Contents[0].Text, "Debian") {
		t.Fatalf("standalone toolbox returned invalid skill resource: %s", responses[7].Result)
	}
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
	testsupport.ServeMockCodex(os.Stdin, os.Stdout, os.Getenv(mockAuthEnvironment) == "1", func() (testsupport.Toolbox, error) {
		return startConfiguredToolbox()
	})
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

type mockToolboxClient struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Scanner
	nextID  int
	tools   map[string]json.RawMessage
	stderr  bytes.Buffer
}

func (c *mockToolboxClient) Tools() map[string]json.RawMessage { return c.tools }

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

func waitForOutput(t *testing.T, output *testsupport.LockedBuffer, text string) {
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
