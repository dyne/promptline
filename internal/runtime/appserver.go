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
func (a AppServer) Account(ctx context.Context) ([]byte, error) { return a.API.Account(ctx) }
func (a AppServer) StartThread(ctx context.Context, cwd, model string) (appserver.Thread, error) {
	return a.API.StartThread(ctx, cwd, model)
}
func (a AppServer) ResumeThread(ctx context.Context, id string) (appserver.Thread, error) {
	return a.API.ResumeThread(ctx, id)
}
func (a AppServer) StartTurn(ctx context.Context, thread, text, message string) (appserver.Turn, error) {
	return a.API.StartTurn(ctx, thread, text, message)
}
func (a AppServer) Interrupt(ctx context.Context, thread, turn string) error {
	return a.API.Interrupt(ctx, thread, turn)
}
func (a AppServer) Events() <-chan appserver.Event           { return a.Client.Events() }
func (a AppServer) Requests() <-chan appserver.ServerRequest { return a.Client.Requests() }
func (a AppServer) ReplyRequest(ctx context.Context, id uint64, decision any) error {
	return a.API.ReplyRequest(ctx, id, decision)
}
func (a AppServer) Done() <-chan struct{} { return a.Client.Done() }
func (a AppServer) Err() error            { return a.Client.Err() }

var _ = json.RawMessage{}
