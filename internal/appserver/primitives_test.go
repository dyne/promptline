package appserver

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestAPI_Lifecycle(t *testing.T) {
	fromClient, toServer := io.Pipe()
	fromServer, toClient := io.Pipe()
	c := New(pipeWriteCloser{toServer}, fromServer, Config{})
	defer c.Close()
	type modelParams struct {
		Method       string
		Model        string
		Instructions string
		DynamicTools []DynamicToolNamespace
	}
	models := make(chan modelParams, 3)
	go func() {
		s := bufio.NewScanner(fromClient)
		for s.Scan() {
			var e envelope
			if json.Unmarshal(s.Bytes(), &e) != nil || e.ID == nil {
				continue
			}
			var result string
			switch e.Method {
			case "initialize":
				result = `{}`
			case "account/read":
				result = `{"account":{"type":"chatgpt"},"requiresOpenaiAuth":true}`
			case "thread/start", "thread/resume", "thread/read":
				if e.Method == "thread/start" || e.Method == "thread/resume" {
					var params struct {
						Model        string                 `json:"model"`
						Instructions string                 `json:"developerInstructions"`
						DynamicTools []DynamicToolNamespace `json:"dynamicTools"`
					}
					_ = json.Unmarshal(e.Params, &params)
					models <- modelParams{
						Method: e.Method, Model: params.Model, Instructions: params.Instructions, DynamicTools: params.DynamicTools,
					}
				}
				result = `{"thread":{"id":"thr_1","status":{"type":"idle"}}}`
			case "turn/start":
				var params struct {
					Model string `json:"model"`
				}
				_ = json.Unmarshal(e.Params, &params)
				models <- modelParams{Method: e.Method, Model: params.Model}
				result = `{"turn":{"id":"turn_1","status":"inProgress"}}`
			case "mcpServerStatus/list":
				result = `{"data":[{"name":"promptline-toolbox","tools":{"ls":{},"pwd":{},"cat":{}}}],"nextCursor":null}`
			case "mcpServer/tool/call":
				result = `{"content":[{"type":"text","text":"ok"}]}`
			default:
				result = `{}`
			}
			_, _ = toClient.Write([]byte(`{"id":` + jsonNumber(*e.ID) + `,"result":` + result + "}\n"))
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	a := NewAPI(c)
	if err := a.Initialize(ctx, Initialize{ClientName: "promptline", ClientVersion: "v2", Experimental: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.Initialize(ctx, Initialize{}); err == nil {
		t.Fatal("double initialize accepted")
	}
	account, err := a.Account(ctx)
	if err != nil || !account.Authenticated() || account.Type != "chatgpt" {
		t.Fatalf("account=%+v err=%v", account, err)
	}
	dynamicTools := []DynamicToolNamespace{{Type: "namespace", Name: "toolbox", Description: "tools", Tools: []DynamicTool{{Type: "function", Name: "pwd", Description: "pwd", InputSchema: map[string]any{"type": "object"}}}}}
	thread, err := a.StartThread(ctx, "/tmp", "gpt-5.6-terra", "prefer toolbox", dynamicTools)
	if err != nil || thread.ID != "thr_1" {
		t.Fatalf("thread=%+v err=%v", thread, err)
	}
	resumed, err := a.ResumeThread(ctx, thread.ID, "gpt-5.6-terra", "prefer toolbox", dynamicTools)
	if err != nil || resumed.ID != "thr_1" {
		t.Fatalf("resumed thread=%+v err=%v", resumed, err)
	}
	turn, err := a.StartTurn(ctx, thread.ID, "hello", "client_1", "gpt-5.6-terra")
	if err != nil || turn.ID != "turn_1" {
		t.Fatalf("turn=%+v err=%v", turn, err)
	}
	if err := a.Interrupt(ctx, thread.ID, turn.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.Unsubscribe(ctx, thread.ID); err != nil {
		t.Fatal(err)
	}
	servers, err := a.ListMCPServers(ctx, thread.ID)
	if err != nil || len(servers) != 1 || servers[0].Name != "promptline-toolbox" {
		t.Fatalf("MCP servers = %+v, err = %v", servers, err)
	}
	if _, ok := servers[0].Tools["ls"]; !ok {
		t.Fatalf("MCP toolbox tools = %v", servers[0].Tools)
	}
	toolResult, err := a.CallMCPTool(ctx, thread.ID, "promptline-toolbox", "pwd", json.RawMessage(`{}`))
	if err != nil || len(toolResult.Content) != 1 {
		t.Fatalf("MCP tool result = %+v, err = %v", toolResult, err)
	}
	for _, method := range []string{"thread/start", "thread/resume", "turn/start"} {
		select {
		case got := <-models:
			if got.Method != method || got.Model != "gpt-5.6-terra" {
				t.Fatalf("model params = %+v, want method %q with gpt-5.6-terra", got, method)
			}
			if method != "turn/start" && got.Instructions != "prefer toolbox" {
				t.Fatalf("developer instructions for %s = %q", method, got.Instructions)
			}
			if method != "turn/start" && len(got.DynamicTools) != 1 {
				t.Fatalf("dynamic tools for %s = %+v", method, got.DynamicTools)
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s parameters", method)
		}
	}
}

func TestDecodeAccountAuthenticationState(t *testing.T) {
	tests := []struct {
		name          string
		result        string
		authenticated bool
	}{
		{
			name:          "authenticated ChatGPT account",
			result:        `{"account":{"type":"chatgpt"},"requiresOpenaiAuth":true}`,
			authenticated: true,
		},
		{
			name:          "missing required OpenAI account",
			result:        `{"account":null,"requiresOpenaiAuth":true}`,
			authenticated: false,
		},
		{
			name:          "provider does not require OpenAI account",
			result:        `{"account":null,"requiresOpenaiAuth":false}`,
			authenticated: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, err := DecodeAccount(json.RawMessage(tt.result))
			if err != nil {
				t.Fatal(err)
			}
			if got := account.Authenticated(); got != tt.authenticated {
				t.Fatalf("Authenticated() = %v, want %v", got, tt.authenticated)
			}
		})
	}
}

func TestAPI_RejectsUninitialized(t *testing.T) {
	c := New(pipeWriteCloser{io.Discard}, &emptyReader{}, Config{})
	defer c.Close()
	a := NewAPI(c)
	ctx := context.Background()
	if _, err := a.ReadThread(ctx, "thr"); err != ErrNotInitialized {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeItemAndReplyOnce(t *testing.T) {
	item, err := DecodeItem(json.RawMessage(`{"threadId":"thr","turnId":"turn","item":{"id":"item","type":"agentMessage","text":"hi","newField":true}}`))
	if err != nil || item.ID != "item" || item.ThreadID != "thr" {
		t.Fatalf("item=%+v err=%v", item, err)
	}
	inR, inW := io.Pipe()
	outR, _ := io.Pipe()
	c := New(pipeWriteCloser{inW}, outR, Config{})
	defer c.Close()
	a := NewAPI(c)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	read := make(chan string, 1)
	go func() {
		buf := make([]byte, 256)
		n, _ := inR.Read(buf)
		read <- string(buf[:n])
	}()
	if err := a.ReplyRequest(ctx, 7, map[string]string{"decision": "decline"}); err != nil {
		t.Fatal(err)
	}
	if err := a.ReplyRequest(ctx, 7, map[string]string{}); err == nil {
		t.Fatal("duplicate reply accepted")
	}
	if got := <-read; !strings.Contains(got, `"id":7`) {
		t.Fatalf("reply=%s", got)
	}
}

func TestDecodeStreamingEvents(t *testing.T) {
	delta, err := DecodeAgentMessageDelta(json.RawMessage(`{"itemId":"item-1","delta":"hello"}`))
	if err != nil || delta.ItemID != "item-1" || delta.Delta != "hello" {
		t.Fatalf("delta=%+v err=%v", delta, err)
	}
	completion, err := DecodeTurnCompletion(json.RawMessage(`{
		"turn": {
			"status": "completed",
			"items": [{"id":"item-1","type":"agentMessage","text":"hello world"}]
		}
	}`))
	if err != nil || completion.Status != "completed" || completion.FinalMessage.Text != "hello world" {
		t.Fatalf("completion=%+v err=%v", completion, err)
	}
	message, err := DecodeErrorMessage(json.RawMessage(`{"error":{"message":"quota exceeded"}}`))
	if err != nil || message != "quota exceeded" {
		t.Fatalf("message=%q err=%v", message, err)
	}
}

func TestDecoders_RejectMalformedAndRetainAdditiveData(t *testing.T) {
	tests := []struct {
		name string
		fn   func(json.RawMessage) error
		data json.RawMessage
	}{
		{name: "item missing identity", fn: func(b json.RawMessage) error { _, err := DecodeItem(b); return err }, data: json.RawMessage(`{"item":{"type":"message"}}`)},
		{name: "item invalid JSON", fn: func(b json.RawMessage) error { _, err := DecodeItem(b); return err }, data: json.RawMessage(`{`)},
		{name: "delta missing ID", fn: func(b json.RawMessage) error { _, err := DecodeAgentMessageDelta(b); return err }, data: json.RawMessage(`{"delta":"x"}`)},
		{name: "error missing message", fn: func(b json.RawMessage) error { _, err := DecodeErrorMessage(b); return err }, data: json.RawMessage(`{"error":{"code":5}}`)},
		{name: "completion invalid JSON", fn: func(b json.RawMessage) error { _, err := DecodeTurnCompletion(b); return err }, data: json.RawMessage(`[`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(tt.data); err == nil {
				t.Fatal("decoder accepted malformed data")
			}
		})
	}
	item, err := DecodeItem(json.RawMessage(`{"item":{"id":"i","type":"agentMessage","new":{"nested":true}}}`))
	if err != nil || !strings.Contains(string(item.Raw), `"nested":true`) {
		t.Fatalf("item=%+v err=%v", item, err)
	}
}

func TestAPI_ReplyRequestWriteFailureIsNotRetried(t *testing.T) {
	out, _ := io.Pipe()
	c := New(pipeWriteCloser{errWriter{}}, out, Config{})
	defer c.Close()
	a := NewAPI(c)
	if err := a.ReplyRequest(context.Background(), 42, map[string]bool{"allow": true}); err == nil {
		t.Fatal("first reply succeeded")
	}
	if err := a.ReplyRequest(context.Background(), 42, map[string]bool{"allow": true}); err == nil {
		t.Fatal("failed reply was retried")
	}
}

func FuzzAppServerDecoders(f *testing.F) {
	for _, seed := range []string{
		`{"item":{"id":"item","type":"agentMessage","text":"hi"}}`,
		`{"requiresOpenaiAuth":true,"account":null}`,
		`{"turn":{"status":{"type":"completed"},"error":{"message":"nested"}}}`,
		`{"error":{"message":"bad"}}`, `null`, `{`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		if len(payload) > 16<<10 {
			t.Skip()
		}
		b := json.RawMessage(payload)
		_, _ = DecodeItem(b)
		_, _ = DecodeAccount(b)
		_, _ = DecodeAgentMessageDelta(b)
		_, _ = DecodeTurnCompletion(b)
		_, _ = DecodeErrorMessage(b)
	})
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) { return 0, io.EOF }
