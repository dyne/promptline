//go:build integration && unix

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const mockCodexEnvironment = "PROMPTLINE_MOCK_CODEX"

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
	mockCodex := writeMockCodexExecutable(t)
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
				"--codex", mockCodex,
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
	config, err := os.ReadFile(filepath.Join(stateRoot, "integration", "codex-home", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(config, []byte("[mcp_servers.promptline-toolbox]")) {
		t.Fatalf("toolbox MCP configuration was not installed: %s", config)
	}
}

func TestToolboxServesBasicURootTools(t *testing.T) {
	work := t.TempDir()
	stateRoot := filepath.Join(t.TempDir(), "instances")
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
		[]string{
			"toolbox", "serve",
			"--instance", "toolbox-integration",
			"--cwd", work,
			"--state-root", stateRoot,
		},
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
}

func TestMockCodexProcess(t *testing.T) {
	if os.Getenv(mockCodexEnvironment) != "1" {
		return
	}
	if slicesContain(os.Args, "--version") {
		fmt.Println("codex-cli 0.147.0")
		os.Exit(0)
	}
	if !slicesContain(os.Args, "app-server") || !slicesContain(os.Args, "--stdio") {
		os.Exit(2)
	}
	serveMockCodex()
	os.Exit(0)
}

func writeMockCodexExecutable(t *testing.T) string {
	t.Helper()
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "codex")
	script := fmt.Sprintf(
		"#!/bin/sh\n%s=1 exec %q -test.run '^TestMockCodexProcess$' -- \"$@\"\n",
		mockCodexEnvironment,
		testExecutable,
	)
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func serveMockCodex() {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var request struct {
			ID     *uint64 `json:"id"`
			Method string  `json:"method"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == nil {
			continue
		}
		result := any(map[string]any{})
		switch request.Method {
		case "account/read":
			result = map[string]any{"account": map[string]any{"type": "mock"}}
		case "thread/start":
			result = map[string]any{"thread": map[string]any{
				"id":     "thread-integration",
				"status": map[string]any{"type": "idle"},
			}}
		case "turn/start":
			result = map[string]any{"turn": map[string]any{
				"id":     "turn-integration",
				"status": "inProgress",
			}}
		}
		_ = encoder.Encode(map[string]any{"id": *request.ID, "result": result})
		if request.Method == "turn/start" {
			_ = encoder.Encode(map[string]any{
				"method": "item/completed",
				"params": map[string]any{
					"threadId": "thread-integration",
					"turnId":   "turn-integration",
					"item": map[string]any{
						"id":   "item-integration",
						"type": "agentMessage",
						"text": "mock reply",
					},
				},
			})
			_ = encoder.Encode(map[string]any{
				"method": "turn/completed",
				"params": map[string]any{"turn": map[string]any{"id": "turn-integration"}},
			})
		}
	}
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
