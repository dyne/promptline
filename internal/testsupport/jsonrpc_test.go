package testsupport

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestScriptedJSONRPC(t *testing.T) {
	t.Run("records ordered calls and injects requests", func(t *testing.T) {
		script := NewScriptedJSONRPC(Expectation{Method: "initialize", Result: map[string]bool{"ok": true}})
		input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
		var output bytes.Buffer
		if err := script.Serve(context.Background(), input, &output); err != nil {
			t.Fatal(err)
		}
		calls := script.Calls()
		if len(calls) != 1 || calls[0].Method != "initialize" {
			t.Fatalf("recorded calls = %#v", calls)
		}
		if err := script.InjectRequest(9, "item/tool/call", map[string]string{"tool": "pwd"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), `"method":"item/tool/call"`) {
			t.Fatalf("injected request missing from JSON-RPC output: %s", output.String())
		}
	})

	t.Run("rejects unexpected method", func(t *testing.T) {
		script := NewScriptedJSONRPC(Expectation{Method: "initialize"})
		err := script.Serve(context.Background(), strings.NewReader(`{"id":1,"method":"turn/start"}`+"\n"), &bytes.Buffer{})
		if !errors.Is(err, ErrUnexpectedMethod) {
			t.Fatalf("Serve error = %v", err)
		}
	})

	t.Run("rejects malformed request", func(t *testing.T) {
		err := NewScriptedJSONRPC().Serve(context.Background(), strings.NewReader("not-json\n"), &bytes.Buffer{})
		if !errors.Is(err, ErrMalformedMessage) {
			t.Fatalf("Serve error = %v", err)
		}
	})

	t.Run("times out deterministically", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		err := NewScriptedJSONRPC().WaitForCalls(ctx, 1)
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("WaitForCalls error = %v", err)
		}
	})
}

func TestAppServerRecordsTypedCalls(t *testing.T) {
	server := NewAppServer()
	if _, err := server.StartTurn(context.Background(), "thread", "input", "", "model"); err != nil {
		t.Fatal(err)
	}
	if len(server.Calls) != 1 || server.Calls[0].Method != "turn/start" {
		t.Fatalf("calls = %#v", server.Calls)
	}
	if err := server.ReplyRequest(context.Background(), 1, map[string]string{"decision": "decline"}); err != nil {
		t.Fatal(err)
	}
	if len(server.Replies) != 1 {
		t.Fatalf("replies = %#v", server.Replies)
	}
}
