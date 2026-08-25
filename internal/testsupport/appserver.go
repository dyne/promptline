package testsupport

import (
	"context"
	"encoding/json"

	"promptline/internal/appserver"
)

// AppServer is a deterministic in-memory app-server client for lifecycle
// tests. Callers configure responses directly and inspect its typed Calls.
type AppServer struct {
	EventsCh     chan appserver.Event
	RequestsCh   chan appserver.ServerRequest
	DoneCh       chan struct{}
	AccountValue appserver.Account
	Thread       appserver.Thread
	Turn         appserver.Turn
	Servers      []appserver.MCPServer
	ToolResult   appserver.MCPToolResult
	ErrValue     error
	Calls        []Call
	Replies      []any
}

func NewAppServer() *AppServer {
	return &AppServer{
		EventsCh: make(chan appserver.Event, 16), RequestsCh: make(chan appserver.ServerRequest, 16), DoneCh: make(chan struct{}),
		AccountValue: appserver.Account{Type: "chatgpt", RequiresOpenAIAuth: true}, Thread: appserver.Thread{ID: "thread-test"}, Turn: appserver.Turn{ID: "turn-test"},
	}
}

func (s *AppServer) record(method string, params any) {
	raw, _ := json.Marshal(params)
	s.Calls = append(s.Calls, Call{Method: method, Params: raw})
}
func (s *AppServer) Initialize(_ context.Context, in appserver.Initialize) error {
	s.record("initialize", in)
	return s.ErrValue
}
func (s *AppServer) Account(context.Context) (appserver.Account, error) {
	s.record("account/read", nil)
	return s.AccountValue, s.ErrValue
}
func (s *AppServer) StartThread(_ context.Context, cwd, model, instructions string, tools []appserver.DynamicToolNamespace) (appserver.Thread, error) {
	s.record("thread/start", map[string]any{"cwd": cwd, "model": model, "instructions": instructions, "tools": tools})
	return s.Thread, s.ErrValue
}
func (s *AppServer) ResumeThread(_ context.Context, id, model, instructions string, tools []appserver.DynamicToolNamespace) (appserver.Thread, error) {
	s.record("thread/resume", map[string]any{"id": id, "model": model, "instructions": instructions, "tools": tools})
	return s.Thread, s.ErrValue
}
func (s *AppServer) StartTurn(_ context.Context, thread, text, cwd, model string) (appserver.Turn, error) {
	s.record("turn/start", map[string]string{"thread": thread, "text": text, "cwd": cwd, "model": model})
	return s.Turn, s.ErrValue
}
func (s *AppServer) ListMCPServers(context.Context, string) ([]appserver.MCPServer, error) {
	s.record("mcpServerStatus/list", nil)
	return s.Servers, s.ErrValue
}
func (s *AppServer) CallMCPTool(_ context.Context, thread, server, tool string, args json.RawMessage) (appserver.MCPToolResult, error) {
	s.record("mcpServer/tool/call", map[string]any{"thread": thread, "server": server, "tool": tool, "arguments": args})
	return s.ToolResult, s.ErrValue
}
func (s *AppServer) Interrupt(context.Context, string, string) error {
	s.record("turn/interrupt", nil)
	return s.ErrValue
}
func (s *AppServer) Unsubscribe(context.Context, string) error {
	s.record("thread/unsubscribe", nil)
	return s.ErrValue
}
func (s *AppServer) Events() <-chan appserver.Event           { return s.EventsCh }
func (s *AppServer) Requests() <-chan appserver.ServerRequest { return s.RequestsCh }
func (s *AppServer) ReplyRequest(_ context.Context, _ uint64, reply any) error {
	s.Replies = append(s.Replies, reply)
	return s.ErrValue
}
func (s *AppServer) Done() <-chan struct{} { return s.DoneCh }
func (s *AppServer) Err() error            { return s.ErrValue }
