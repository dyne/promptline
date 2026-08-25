package testsupport

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// Toolbox is the MCP capability used by the scripted Codex app-server.
type Toolbox interface {
	Tools() map[string]json.RawMessage
	CallText(string, map[string]any) (string, error)
	Close() error
}

// ServeMockCodex is a deterministic app-server script for command integration
// tests. Process-specific trampoline code stays in the command test binary.
func ServeMockCodex(input io.Reader, output io.Writer, authenticated bool, startToolbox func() (Toolbox, error)) {
	const dynamicCallID uint64 = 9000
	scanner := bufio.NewScanner(input)
	encoder := json.NewEncoder(output)
	var toolbox Toolbox
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
				emitMockTurn(encoder, reply.ContentItems[0].Text)
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
				encodeError(encoder, *request.ID, -32602, "experimental API unavailable")
				continue
			}
		case "account/read":
			if authenticated {
				result = map[string]any{"account": map[string]any{"type": "mock"}, "requiresOpenaiAuth": true}
			} else {
				result = map[string]any{"account": nil, "requiresOpenaiAuth": true}
			}
		case "thread/start":
			result = map[string]any{"thread": map[string]any{"id": "thread-integration", "status": map[string]any{"type": "idle"}}}
		case "turn/start":
			result = map[string]any{"turn": map[string]any{"id": "turn-integration", "status": "inProgress"}}
		case "mcpServerStatus/list":
			var err error
			toolbox, err = startToolbox()
			if err != nil {
				encodeError(encoder, *request.ID, -32000, err.Error())
				continue
			}
			result = map[string]any{"data": []map[string]any{{"name": "promptline-toolbox", "tools": toolbox.Tools(), "resources": []any{}, "resourceTemplates": []any{}, "authStatus": "unsupported"}}, "nextCursor": nil}
		case "mcpServer/tool/call":
			var params struct {
				Server    string         `json:"server"`
				Tool      string         `json:"tool"`
				Arguments map[string]any `json:"arguments"`
			}
			if json.Unmarshal(request.Params, &params) != nil || params.Server != "promptline-toolbox" || toolbox == nil {
				encodeError(encoder, *request.ID, -32602, "invalid MCP tool call")
				continue
			}
			text, err := toolbox.CallText(params.Tool, params.Arguments)
			if err != nil {
				encodeError(encoder, *request.ID, -32000, err.Error())
				continue
			}
			result = map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
		}
		_ = encoder.Encode(map[string]any{"id": *request.ID, "result": result})
		if request.Method == "turn/start" {
			_ = encoder.Encode(map[string]any{"id": dynamicCallID, "method": "item/tool/call", "params": map[string]any{"threadId": "thread-integration", "turnId": "turn-integration", "callId": "call-integration", "namespace": "toolbox", "tool": "echo", "arguments": map[string]any{"text": "mock reply"}}})
		}
	}
}
func encodeError(e *json.Encoder, id uint64, code int, message string) {
	_ = e.Encode(map[string]any{"id": id, "error": map[string]any{"code": code, "message": message}})
}
func emitMockTurn(e *json.Encoder, reply string) {
	_ = e.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"turn": map[string]any{"id": "turn-integration", "status": "completed", "items": []map[string]any{{"id": "item-integration", "type": "agentMessage", "text": strings.TrimSpace(reply)}}}}})
}
