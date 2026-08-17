// Package mcp implements the deliberately small stable stdio MCP surface used
// to expose Promptline's embedded toolbox. It has no provider dependencies.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"promptline/internal/tools"
)

const protocolVersion = "2024-11-05"

type request struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Server owns one input and output stream. It serves only initialize,
// tools/list and tools/call; unsupported methods receive a JSON-RPC error.
type Server struct {
	registry *tools.Registry
	in       io.Reader
	out      io.Writer
	maxFrame int
	writeMu  sync.Mutex
}

func NewServer(registry *tools.Registry, in io.Reader, out io.Writer, maxFrame int) (*Server, error) {
	if registry == nil || in == nil || out == nil {
		return nil, errors.New("mcp server requires registry, input, and output")
	}
	if maxFrame <= 0 {
		maxFrame = 1 << 20
	}
	return &Server{registry: registry, in: in, out: out, maxFrame: maxFrame}, nil
}

func (s *Server) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 4096), s.maxFrame)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil || req.Method == "" {
			if writeErr := s.reply(json.RawMessage("null"), nil, -32600, "invalid request"); writeErr != nil {
				return writeErr
			}
			continue
		}
		if len(req.ID) == 0 { // Notifications never receive a response.
			continue
		}
		if err := s.handle(ctx, req); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read MCP request: %w", err)
	}
	return nil
}

func (s *Server) handle(ctx context.Context, req request) error {
	switch req.Method {
	case "initialize":
		return s.reply(req.ID, map[string]any{"protocolVersion": protocolVersion, "serverInfo": map[string]string{"name": "promptline-toolbox", "version": "v2"}, "capabilities": map[string]any{"tools": map[string]any{}}}, 0, "")
	case "tools/list":
		return s.reply(req.ID, map[string]any{"tools": s.definitions()}, 0, "")
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil || params.Name == "" {
			return s.reply(req.ID, nil, -32602, "invalid tools/call parameters")
		}
		result := s.registry.ExecuteContext(ctx, params.Name, params.Arguments)
		content := []map[string]string{{"type": "text", "text": result.Result}}
		if result.Error != nil {
			return s.reply(req.ID, map[string]any{"content": content, "isError": true}, 0, "")
		}
		return s.reply(req.ID, map[string]any{"content": content}, 0, "")
	default:
		return s.reply(req.ID, nil, -32601, "method not found")
	}
}

func (s *Server) definitions() []map[string]any {
	registered := s.registry.GetTools()
	definitions := make([]map[string]any, 0, len(registered))
	for _, tool := range registered {
		definitions = append(definitions, map[string]any{
			"name": tool.Name(), "description": tool.Description(), "inputSchema": tool.Parameters(),
		})
	}
	return definitions
}

func (s *Server) reply(id json.RawMessage, result any, code int, message string) error {
	response := map[string]any{"jsonrpc": "2.0", "id": id}
	if code != 0 {
		response["error"] = map[string]any{"code": code, "message": message}
	} else {
		response["result"] = result
	}
	b, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if len(b)+1 > s.maxFrame {
		return errors.New("MCP response exceeds frame limit")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.out.Write(append(b, '\n'))
	return err
}
