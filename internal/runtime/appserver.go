package runtime

import (
	"context"
	"encoding/json"

	"promptline/internal/appserver"
)

// AppServer adapts the protocol API and event transport into the runtime port.
type AppServer struct {
	API    *appserver.API
	Client *appserver.Client
}

func (a AppServer) Initialize(ctx context.Context, in appserver.Initialize) error {
	return a.API.Initialize(ctx, in)
}
func (a AppServer) Account(ctx context.Context) (appserver.Account, error) {
	return a.API.Account(ctx)
}
func (a AppServer) StartThread(ctx context.Context, cwd, model, instructions string) (appserver.Thread, error) {
	return a.API.StartThread(ctx, cwd, model, instructions)
}
func (a AppServer) ResumeThread(ctx context.Context, id, model, instructions string) (appserver.Thread, error) {
	return a.API.ResumeThread(ctx, id, model, instructions)
}
func (a AppServer) StartTurn(ctx context.Context, thread, text, message, model string) (appserver.Turn, error) {
	return a.API.StartTurn(ctx, thread, text, message, model)
}
func (a AppServer) ListMCPServers(ctx context.Context, thread string) ([]appserver.MCPServer, error) {
	return a.API.ListMCPServers(ctx, thread)
}
func (a AppServer) Interrupt(ctx context.Context, thread, turn string) error {
	return a.API.Interrupt(ctx, thread, turn)
}
func (a AppServer) Unsubscribe(ctx context.Context, thread string) error {
	return a.API.Unsubscribe(ctx, thread)
}
func (a AppServer) Events() <-chan appserver.Event           { return a.Client.Events() }
func (a AppServer) Requests() <-chan appserver.ServerRequest { return a.Client.Requests() }
func (a AppServer) ReplyRequest(ctx context.Context, id uint64, decision any) error {
	return a.API.ReplyRequest(ctx, id, decision)
}
func (a AppServer) Done() <-chan struct{} { return a.Client.Done() }
func (a AppServer) Err() error            { return a.Client.Err() }

var _ = json.RawMessage{}
