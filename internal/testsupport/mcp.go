package testsupport

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// MCPResponse is a decoded JSON-RPC response indexed by request ID.
type MCPResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
}

func DecodeMCPResponses(t *testing.T, output []byte) map[int]MCPResponse {
	t.Helper()
	responses := map[int]MCPResponse{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var response MCPResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		responses[response.ID] = response
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return responses
}

func AssertMCPToolList(t *testing.T, response MCPResponse, expected ...string) {
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

func AssertMCPTextResult(t *testing.T, response MCPResponse, expected string) {
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
