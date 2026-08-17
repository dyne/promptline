package appserver

import (
	"encoding/json"
	"errors"
	"fmt"
)

const TestedCLIVersion = "0.147.0"

var (
	ErrClosed         = errors.New("app-server client is closed")
	ErrOverloaded     = errors.New("app-server overloaded")
	ErrProtocol       = errors.New("app-server protocol error")
	ErrNotInitialized = errors.New("app-server client is not initialized")
)

// RPCError is the stable JSON-RPC error projection used by Codex.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("app-server rpc error %d: %s", e.Code, e.Message)
}
func (e *RPCError) Is(target error) bool { return target == ErrOverloaded && e.Code == -32001 }

// envelope intentionally omits jsonrpc: app-server's stdio protocol does too.
type envelope struct {
	ID     *uint64         `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

// Event is an additive-field-tolerant notification from app-server.
type Event struct {
	Method string
	Params json.RawMessage
}

// ServerRequest is a request initiated by app-server (usually an approval).
type ServerRequest struct {
	ID     uint64
	Method string
	Params json.RawMessage
}

type Limits struct{ MaxFrameBytes, MaxPending, MaxEvents, MaxServerRequests int }

func (l Limits) normalized() Limits {
	if l.MaxFrameBytes <= 0 {
		l.MaxFrameBytes = 1 << 20
	}
	if l.MaxPending <= 0 {
		l.MaxPending = 64
	}
	if l.MaxEvents <= 0 {
		l.MaxEvents = 128
	}
	if l.MaxServerRequests <= 0 {
		l.MaxServerRequests = 32
	}
	return l
}

type Config struct{ Limits Limits }
