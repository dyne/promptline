// Package testsupport provides deterministic helpers shared by protocol and
// subprocess tests. It is deliberately small: production packages never use
// these scripted transports.
package testsupport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

var (
	ErrUnexpectedMethod = errors.New("unexpected JSON-RPC method")
	ErrMalformedMessage = errors.New("malformed JSON-RPC message")
	ErrTimeout          = errors.New("scripted JSON-RPC timeout")
)

// Call is a typed record of one JSON-RPC request accepted by a script.
type Call struct {
	ID     json.RawMessage
	Method string
	Params json.RawMessage
}

// Expectation defines the next request and the response returned for it.
type Expectation struct {
	Method string
	Result any
	Error  any
}

// ScriptedJSONRPC validates ordered requests and records them for semantic
// assertions. It may inject server requests through the same protected writer.
type ScriptedJSONRPC struct {
	mu           sync.Mutex
	expectations []Expectation
	calls        []Call
	writer       *json.Encoder
	writeMu      sync.Mutex
}

func NewScriptedJSONRPC(expectations ...Expectation) *ScriptedJSONRPC {
	return &ScriptedJSONRPC{expectations: append([]Expectation(nil), expectations...)}
}

func (s *ScriptedJSONRPC) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

// Serve consumes newline-delimited JSON-RPC until the input closes.
func (s *ScriptedJSONRPC) Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	s.mu.Lock()
	s.writer = json.NewEncoder(output)
	s.mu.Unlock()
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var request Call
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil || request.Method == "" || len(request.ID) == 0 {
			return fmt.Errorf("%w: %s", ErrMalformedMessage, scanner.Text())
		}
		s.mu.Lock()
		if len(s.expectations) == 0 || s.expectations[0].Method != request.Method {
			s.mu.Unlock()
			return fmt.Errorf("%w: got %q", ErrUnexpectedMethod, request.Method)
		}
		expectation := s.expectations[0]
		s.expectations = s.expectations[1:]
		s.calls = append(s.calls, request)
		s.mu.Unlock()
		response := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(request.ID)}
		if expectation.Error != nil {
			response["error"] = expectation.Error
		} else {
			response["result"] = expectation.Result
		}
		if err := s.write(response); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// InjectRequest emits a deterministic server-initiated request after Serve
// has attached an output stream.
func (s *ScriptedJSONRPC) InjectRequest(id uint64, method string, params any) error {
	if method == "" {
		return fmt.Errorf("%w: empty method", ErrMalformedMessage)
	}
	return s.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}

func (s *ScriptedJSONRPC) write(value any) error {
	s.mu.Lock()
	writer := s.writer
	s.mu.Unlock()
	if writer == nil {
		return errors.New("scripted JSON-RPC server is not running")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writer.Encode(value)
}

// WaitForCalls bounds asynchronous protocol assertions with a context-derived
// deadline and returns a typed timeout instead of sleeping in tests.
func (s *ScriptedJSONRPC) WaitForCalls(ctx context.Context, want int) error {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if len(s.Calls()) >= want {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: want %d calls, got %d", ErrTimeout, want, len(s.Calls()))
		case <-ticker.C:
		}
	}
}

// LockedBuffer is safe for test goroutines that concurrently capture output.
type LockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *LockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}
func (b *LockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

// TempPaths gives subprocess tests isolated roots and verifies no cleanup is
// deferred to a repository-local temporary directory.
type TempPaths struct{ Root, StateRoot, CodexHome string }

func NewTempPaths(t *testing.T) TempPaths {
	t.Helper()
	root := t.TempDir()
	return TempPaths{Root: root, StateRoot: root + "/instances", CodexHome: root + "/codex-home"}
}
