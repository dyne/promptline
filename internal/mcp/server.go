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
	"path"
	"strings"
	"sync"

	"promptline/internal/tools"
	"promptline/plugins/promptline/skills"
)

const protocolVersion = "2024-11-05"

type request struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Server owns one input and output stream. It serves the Promptline tools and
// the embedded skill resources; unsupported methods receive a JSON-RPC error.
type Server struct {
	registry *tools.Registry
	catalog  *skills.Catalog
	in       io.Reader
	out      io.Writer
	maxFrame int
	writeMu  sync.Mutex
}

func NewServer(registry *tools.Registry, in io.Reader, out io.Writer, maxFrame int) (*Server, error) {
	catalog, err := skills.EmbeddedCatalog()
	if err != nil {
		return nil, fmt.Errorf("load embedded skill catalog: %w", err)
	}
	return NewServerWithCatalog(registry, catalog, in, out, maxFrame)
}

// NewServerWithCatalog builds a server using catalog. It is primarily a narrow
// injection seam for protocol tests; production callers use NewServer.
func NewServerWithCatalog(registry *tools.Registry, catalog *skills.Catalog, in io.Reader, out io.Writer, maxFrame int) (*Server, error) {
	if registry == nil || catalog == nil || in == nil || out == nil {
		return nil, errors.New("mcp server requires registry, catalog, input, and output")
	}
	if maxFrame <= 0 {
		maxFrame = 1 << 20
	}
	return &Server{registry: registry, catalog: catalog, in: in, out: out, maxFrame: maxFrame}, nil
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
		return s.reply(req.ID, map[string]any{"protocolVersion": protocolVersion, "serverInfo": map[string]string{"name": "promptline-toolbox", "version": "v2"}, "capabilities": map[string]any{"tools": map[string]any{}, "resources": map[string]any{}}}, 0, "")
	case "tools/list":
		return s.reply(req.ID, map[string]any{"tools": s.definitions()}, 0, "")
	case "resources/list":
		return s.listResources(req)
	case "resources/read":
		return s.readResource(req)
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil || params.Name == "" {
			return s.reply(req.ID, nil, -32602, "invalid tools/call parameters")
		}
		result := s.registry.ExecuteContext(ctx, params.Name, params.Arguments)
		if result.Error != nil {
			content := []map[string]string{{"type": "text", "text": result.Error.Error()}}
			return s.reply(req.ID, map[string]any{"content": content, "isError": true}, 0, "")
		}
		content := []map[string]string{{"type": "text", "text": result.Result}}
		return s.reply(req.ID, map[string]any{"content": content}, 0, "")
	default:
		return s.reply(req.ID, nil, -32601, "method not found")
	}
}

func (s *Server) listResources(req request) error {
	var params struct {
		Cursor *string `json:"cursor"`
	}
	if len(req.Params) > 0 && string(req.Params) != "null" && decodeParams(req.Params, &params) != nil {
		return s.reply(req.ID, nil, -32602, "invalid resources/list parameters")
	}
	if params.Cursor != nil {
		return s.reply(req.ID, nil, -32602, "resources/list cursors are not supported")
	}
	resources := make([]map[string]string, 0)
	for _, skill := range s.catalog.ListSkills() {
		files, err := s.catalog.ListFiles(skill)
		if err != nil {
			return fmt.Errorf("list embedded skill files: %w", err)
		}
		for _, file := range files {
			uri, err := s.catalog.URI(skill, file)
			if err != nil {
				return fmt.Errorf("build embedded skill URI: %w", err)
			}
			resources = append(resources, map[string]string{
				"uri": uri, "name": skill + "/" + file, "title": skill + ": " + file, "mimeType": resourceMIMEType(file),
			})
		}
	}
	return s.reply(req.ID, map[string]any{"resources": resources}, 0, "")
}

func (s *Server) readResource(req request) error {
	var params struct {
		URI string `json:"uri"`
	}
	if err := decodeParams(req.Params, &params); err != nil || params.URI == "" {
		return s.reply(req.ID, nil, -32602, "invalid resources/read parameters")
	}
	skill, file, err := s.catalog.ParseURI(params.URI)
	if err != nil {
		return s.reply(req.ID, nil, -32602, "invalid embedded skill resource URI")
	}
	content, err := s.catalog.ReadFile(skill, file)
	if err != nil {
		return s.reply(req.ID, nil, -32602, "invalid embedded skill resource URI")
	}
	result := map[string]any{"contents": []map[string]string{{
		"uri": params.URI, "mimeType": resourceMIMEType(file), "text": string(content),
	}}}
	if s.responseExceedsFrame(req.ID, result) {
		return s.reply(req.ID, nil, -32000, "resource response exceeds frame limit")
	}
	return s.reply(req.ID, result, 0, "")
}

func decodeParams(raw json.RawMessage, destination any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return errors.New("missing parameters")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return errors.New("parameters must be an object")
	}
	return json.Unmarshal(raw, destination)
}

func resourceMIMEType(file string) string {
	switch strings.ToLower(path.Ext(file)) {
	case ".md":
		return "text/markdown"
	case ".yaml", ".yml":
		return "application/yaml"
	default:
		return "text/plain"
	}
}

func (s *Server) responseExceedsFrame(id json.RawMessage, result any) bool {
	response := map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
	encoded, err := json.Marshal(response)
	return err != nil || len(encoded)+1 > s.maxFrame
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
