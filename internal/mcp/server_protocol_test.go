package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"promptline/internal/tools"
)

func TestServerProtocolFailuresAndNotifications(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode string
		wantNone bool
	}{
		{name: "notification has no reply", input: `{"jsonrpc":"2.0","method":"initialized","params":{}}` + "\n", wantNone: true},
		{name: "unknown method", input: `{"jsonrpc":"2.0","id":"x","method":"unknown"}` + "\n", wantCode: "-32601"},
		{name: "malformed request", input: `not-json` + "\n", wantCode: "-32600"},
		{name: "missing method", input: `{"jsonrpc":"2.0","id":1}` + "\n", wantCode: "-32600"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tools.NewRegistry()
			t.Cleanup(func() { _ = registry.Close() })
			var output bytes.Buffer
			server, err := NewServer(registry, strings.NewReader(tt.input), &output, 4096)
			if err != nil {
				t.Fatal(err)
			}
			if err := server.Serve(context.Background()); err != nil {
				t.Fatal(err)
			}
			if tt.wantNone {
				if output.Len() != 0 {
					t.Fatalf("notification replied: %s", output.String())
				}
				return
			}
			if !strings.Contains(output.String(), tt.wantCode) {
				t.Fatalf("response = %s, want code %s", output.String(), tt.wantCode)
			}
		})
	}
}

func TestServerFrameAndContextBounds(t *testing.T) {
	registry := tools.NewRegistry()
	t.Cleanup(func() { _ = registry.Close() })
	server, err := NewServer(registry, strings.NewReader(strings.Repeat("x", 128)+"\n"), &bytes.Buffer{}, 32)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background()); err == nil {
		t.Fatal("oversized request was accepted")
	}

	var output bytes.Buffer
	server, err = NewServer(registry, strings.NewReader(`{"id":1,"method":"tools/list"}`+"\n"), &output, 32)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(context.Background()); err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("small output frame error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server, err = NewServer(registry, strings.NewReader(`{"id":1,"method":"initialize"}`+"\n"), &bytes.Buffer{}, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Serve(ctx); err == nil {
		t.Fatal("cancelled server accepted request")
	}
}

func FuzzServerRejectsMalformedRequests(f *testing.F) {
	for _, seed := range []string{"", "{}", "not-json", `{"id":1}`, `{"id":1,"method":"tools/call","params":[]}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			t.Skip()
		}
		registry := tools.NewRegistry()
		t.Cleanup(func() { _ = registry.Close() })
		server, err := NewServer(registry, strings.NewReader(input+"\n"), &bytes.Buffer{}, 4096)
		if err != nil {
			t.Fatal(err)
		}
		_ = server.Serve(context.Background())
	})
}
