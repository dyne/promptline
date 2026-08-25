package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"promptline/internal/appserver"
	"promptline/internal/instance"
)

type fakeClient struct {
	events                                                          chan appserver.Event
	requests                                                        chan appserver.ServerRequest
	done                                                            chan struct{}
	startThreads, resumes, turns, interrupts, unsubscribes, replies int
	resumeErr                                                       error
	initializeWait                                                  bool
	account                                                         appserver.Account
	threadModel, turnModel                                          string
	threadInstructions, resumeInstructions                          string
	mcpServers                                                      []appserver.MCPServer
	mcpLists                                                        int
	dynamicTools                                                    []appserver.DynamicToolNamespace
	experimental                                                    bool
	mcpCalls                                                        int
	mcpResult                                                       appserver.MCPToolResult
	lastReply                                                       any
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		events:   make(chan appserver.Event, 8),
		requests: make(chan appserver.ServerRequest, 8),
		done:     make(chan struct{}),
		account:  appserver.Account{Type: "chatgpt", RequiresOpenAIAuth: true},
		mcpServers: []appserver.MCPServer{{
			Name: "promptline-toolbox",
			Tools: map[string]json.RawMessage{
				"ls": {}, "pwd": {}, "cat": {},
			},
		}},
	}
}
func (f *fakeClient) Initialize(ctx context.Context, in appserver.Initialize) error {
	f.experimental = in.Experimental
	if f.initializeWait {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}
func (f *fakeClient) Account(context.Context) (appserver.Account, error) { return f.account, nil }
func (f *fakeClient) StartThread(_ context.Context, _, model, instructions string, dynamicTools []appserver.DynamicToolNamespace) (appserver.Thread, error) {
	f.startThreads++
	f.threadModel = model
	f.threadInstructions = instructions
	f.dynamicTools = dynamicTools
	return appserver.Thread{ID: "new-thread"}, nil
}
func (f *fakeClient) ResumeThread(_ context.Context, _, _, instructions string, dynamicTools []appserver.DynamicToolNamespace) (appserver.Thread, error) {
	f.resumes++
	f.resumeInstructions = instructions
	f.dynamicTools = dynamicTools
	if f.resumeErr != nil {
		return appserver.Thread{}, f.resumeErr
	}
	return appserver.Thread{ID: "stored-thread"}, nil
}
func (f *fakeClient) ListMCPServers(context.Context, string) ([]appserver.MCPServer, error) {
	f.mcpLists++
	return f.mcpServers, nil
}
func (f *fakeClient) CallMCPTool(context.Context, string, string, string, json.RawMessage) (appserver.MCPToolResult, error) {
	f.mcpCalls++
	return f.mcpResult, nil
}
func (f *fakeClient) StartTurn(_ context.Context, _, _, _, model string) (appserver.Turn, error) {
	f.turns++
	f.turnModel = model
	return appserver.Turn{ID: "turn-1"}, nil
}
func (f *fakeClient) Interrupt(context.Context, string, string) error { f.interrupts++; return nil }
func (f *fakeClient) Unsubscribe(context.Context, string) error       { f.unsubscribes++; return nil }
func (f *fakeClient) Events() <-chan appserver.Event                  { return f.events }
func (f *fakeClient) Requests() <-chan appserver.ServerRequest        { return f.requests }
func (f *fakeClient) ReplyRequest(_ context.Context, _ uint64, reply any) error {
	f.replies++
	f.lastReply = reply
	return nil
}
func (f *fakeClient) Done() <-chan struct{} { return f.done }
func (f *fakeClient) Err() error            { return nil }

type fakeProcess struct {
	closed       int
	codexVersion string
}

func (p *fakeProcess) CodexVersion() string        { return p.codexVersion }
func (p *fakeProcess) Close(context.Context) error { p.closed++; return nil }

type blockingProcess struct{}

func (*blockingProcess) CodexVersion() string { return "" }
func (*blockingProcess) Close(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

type fakeRenderer struct{ strings.Builder }

func (r *fakeRenderer) Prompt() error           { _, e := r.WriteString("> "); return e }
func (r *fakeRenderer) Delta(s string) error    { _, e := r.WriteString(s); return e }
func (r *fakeRenderer) Text(s string) error     { _, e := r.WriteString(s); return e }
func (r *fakeRenderer) Progress(s string) error { _, e := r.WriteString(s); return e }
func (r *fakeRenderer) Error(e error) error     { _, e2 := r.WriteString(e.Error()); return e2 }

func testRuntime(t *testing.T) (*Runtime, *fakeClient, *fakeProcess) {
	t.Helper()
	in, err := instance.New(instance.Config{
		Name: "test", StateRoot: t.TempDir(), WorkingRoot: t.TempDir(), ToolboxEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := in.AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	f, p := newFakeClient(), &fakeProcess{codexVersion: "0.149.0"}
	r, err := New(in, f, p, lock)
	if err != nil {
		t.Fatal(err)
	}
	return r, f, p
}

func TestRuntime_StartSelectsOneThread(t *testing.T) {
	r, f, _ := testRuntime(t)
	defer r.Close(context.Background())
	dynamicTools := []appserver.DynamicToolNamespace{{Type: "namespace", Name: "toolbox"}}
	if err := r.Start(context.Background(), Options{DynamicTools: dynamicTools}, "test"); err != nil {
		t.Fatal(err)
	}
	if f.startThreads != 1 || r.ThreadID() != "new-thread" {
		t.Fatalf("start=%d thread=%q", f.startThreads, r.ThreadID())
	}
	if f.threadModel != instance.DefaultModel {
		t.Fatalf("thread model = %q, want %q", f.threadModel, instance.DefaultModel)
	}
	if !strings.Contains(f.threadInstructions, "promptline-toolbox") || f.mcpLists != 1 {
		t.Fatalf("toolbox instructions = %q, MCP lists = %d", f.threadInstructions, f.mcpLists)
	}
	if !f.experimental || len(f.dynamicTools) != 1 || f.dynamicTools[0].Name != "toolbox" {
		t.Fatalf("experimental=%v dynamic tools=%+v", f.experimental, f.dynamicTools)
	}
	state, err := r.instance.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.CodexVersion != "0.149.0" {
		t.Fatalf("recorded Codex version = %q, want 0.149.0", state.CodexVersion)
	}
	if _, err := r.StartTurn(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if f.turnModel != instance.DefaultModel {
		t.Fatalf("turn model = %q, want %q", f.turnModel, instance.DefaultModel)
	}
	if _, err := r.StartTurn(context.Background(), "again"); !errors.Is(err, ErrActiveTurn) {
		t.Fatalf("got %v", err)
	}
	if err := r.Interrupt(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.interrupts != 1 {
		t.Fatalf("interrupts=%d", f.interrupts)
	}
}

func TestRuntimeRejectsMissingAuthenticationBeforeStartingThread(t *testing.T) {
	r, client, _ := testRuntime(t)
	defer r.Close(context.Background())
	client.account = appserver.Account{RequiresOpenAIAuth: true}
	err := r.Start(context.Background(), Options{Resume: true}, "test")
	if !errors.Is(err, ErrAuthenticationRequired) {
		t.Fatalf("Start() error = %v, want authentication required", err)
	}
	if client.startThreads != 0 || client.resumes != 0 {
		t.Fatalf("unauthenticated startup created or resumed a thread: start=%d resume=%d", client.startThreads, client.resumes)
	}
	for _, expected := range []string{"CODEX_HOME=", "codex", "login", "restart Promptline"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("authentication error is missing %q: %v", expected, err)
		}
	}
}

func TestRuntimeRejectsMissingToolboxBeforePersistingThread(t *testing.T) {
	r, client, _ := testRuntime(t)
	defer r.Close(context.Background())
	client.mcpServers = nil
	err := r.Start(context.Background(), Options{}, "test")
	if !errors.Is(err, ErrToolboxUnavailable) {
		t.Fatalf("Start() error = %v, want toolbox unavailable", err)
	}
	if client.startThreads != 1 || client.mcpLists != 1 {
		t.Fatalf("start threads = %d, MCP lists = %d", client.startThreads, client.mcpLists)
	}
	state, loadErr := r.instance.LoadState()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if state.LastPrimaryThreadID != "" {
		t.Fatalf("unverified toolbox persisted thread %q", state.LastPrimaryThreadID)
	}
}

func TestRuntime_StoredThreadResumeFailureDoesNotCreateThread(t *testing.T) {
	r, f, _ := testRuntime(t)
	defer r.Close(context.Background())
	if err := r.instance.SaveState(instance.State{LastPrimaryThreadID: "old"}); err != nil {
		t.Fatal(err)
	}
	f.resumeErr = errors.New("missing")
	err := r.Start(context.Background(), Options{Resume: true}, "test")
	if !errors.Is(err, ErrResumeFailed) || f.startThreads != 0 {
		t.Fatalf("err=%v start=%d", err, f.startThreads)
	}
}

func TestRuntimeResumeRequiresSavedOrExplicitThread(t *testing.T) {
	r, client, _ := testRuntime(t)
	defer r.Close(context.Background())
	err := r.Start(context.Background(), Options{Resume: true}, "test")
	if !errors.Is(err, ErrNoStoredThread) {
		t.Fatalf("Start() error = %v, want no stored thread", err)
	}
	if client.startThreads != 0 || client.resumes != 0 {
		t.Fatalf("resume without state started=%d resumed=%d", client.startThreads, client.resumes)
	}
}

func TestRuntime_RunRendersTurnAndClosesOnEOF(t *testing.T) {
	r, f, p := testRuntime(t)
	if err := r.Start(context.Background(), Options{}, "test"); err != nil {
		t.Fatal(err)
	}
	f.events <- appserver.Event{Method: "turn/completed"}
	render := &fakeRenderer{}
	if err := r.Run(context.Background(), strings.NewReader("hello\n"), render); err != nil {
		t.Fatal(err)
	}
	if f.turns != 1 || f.unsubscribes != 1 || p.closed != 1 {
		t.Fatalf("turns=%d unsubscribes=%d closed=%d", f.turns, f.unsubscribes, p.closed)
	}
}

func TestRuntime_ItemCompletionDoesNotCompleteTurn(t *testing.T) {
	r, _, _ := testRuntime(t)
	defer r.Close(context.Background())
	if err := r.Start(context.Background(), Options{}, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.StartTurn(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	render := &fakeRenderer{}
	item := appserver.Event{
		Method: "item/completed",
		Params: []byte(`{"item":{"id":"item-1","turnId":"turn-1","type":"agentMessage","text":"answer"}}`),
	}
	if err := r.renderEvent(item, render); err != nil {
		t.Fatal(err)
	}
	if !r.HasActiveTurn() {
		t.Fatal("item completion ended the active turn")
	}
	if !strings.Contains(render.String(), "answer") {
		t.Fatalf("completed item was not rendered: %q", render.String())
	}
	if err := r.renderEvent(appserver.Event{Method: "turn/completed"}, render); err != nil {
		t.Fatal(err)
	}
	if r.HasActiveTurn() {
		t.Fatal("turn completion left the turn active")
	}
}

func TestRuntimeStreamsAgentOutputWithoutDuplicatingCompletion(t *testing.T) {
	r, _, _ := testRuntime(t)
	defer r.Close(context.Background())
	if err := r.Start(context.Background(), Options{}, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.StartTurn(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	render := &fakeRenderer{}
	events := []appserver.Event{
		{Method: "item/agentMessage/delta", Params: []byte(`{"itemId":"item-1","delta":"hello "}`)},
		{Method: "item/agentMessage/delta", Params: []byte(`{"itemId":"item-1","delta":"world"}`)},
		{Method: "item/completed", Params: []byte(`{"item":{"id":"item-1","type":"agentMessage","text":"hello world"}}`)},
		{Method: "turn/completed", Params: []byte(`{"turn":{"status":"completed","items":[{"id":"item-1","type":"agentMessage","text":"hello world"}]}}`)},
	}
	for _, event := range events {
		if err := r.renderEvent(event, render); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(render.String(), "hello world"); got != 1 {
		t.Fatalf("rendered response %d times: %q", got, render.String())
	}
}

func TestRuntimeRendersTurnFallbackAndErrors(t *testing.T) {
	r, _, _ := testRuntime(t)
	defer r.Close(context.Background())
	if err := r.Start(context.Background(), Options{}, "test"); err != nil {
		t.Fatal(err)
	}
	render := &fakeRenderer{}
	completed := appserver.Event{
		Method: "turn/completed",
		Params: []byte(`{"turn":{"status":"completed","items":[{"id":"item-1","type":"agentMessage","text":"fallback"}]}}`),
	}
	if err := r.renderEvent(completed, render); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(render.String(), "fallback") {
		t.Fatalf("final message fallback was not rendered: %q", render.String())
	}
	render.Reset()
	if err := r.renderEvent(appserver.Event{Method: "error", Params: []byte(`{"error":{"message":"quota exceeded"}}`)}, render); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(render.String(), "quota exceeded") {
		t.Fatalf("turn error was not rendered: %q", render.String())
	}
}

func TestRuntime_CloseIsIdempotent(t *testing.T) {
	r, _, p := testRuntime(t)
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p.closed != 1 {
		t.Fatalf("process closed %d times", p.closed)
	}
	if r.api.(*fakeClient).unsubscribes != 0 {
		t.Fatal("runtime without a selected thread unsubscribed")
	}
}

func TestRuntimeStartupUsesInstanceTimeout(t *testing.T) {
	r, client, _ := testRuntimeWithTimeouts(t, instance.Timeouts{Startup: 10 * time.Millisecond})
	client.initializeWait = true
	started := time.Now()
	err := r.Start(context.Background(), Options{}, "test")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("start error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("startup timeout took %v", elapsed)
	}
	_ = r.Close(context.Background())
}

func TestRuntimeCloseUsesInstanceTimeout(t *testing.T) {
	in, err := instance.New(instance.Config{
		Name: "close-timeout", StateRoot: t.TempDir(), WorkingRoot: t.TempDir(),
		Timeouts: instance.Timeouts{Shutdown: 10 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := in.AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(in, newFakeClient(), &blockingProcess{}, lock)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = r.Close(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("close error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("shutdown timeout took %v", elapsed)
	}
}

func TestRuntimeDeclinesUnhandledServerRequest(t *testing.T) {
	r, f, _ := testRuntime(t)
	defer r.Close(context.Background())
	if err := r.handleRequest(context.Background(), appserver.ServerRequest{ID: 7, Method: "item/commandExecution/requestApproval"}, strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	if f.replies != 1 {
		t.Fatalf("replies=%d, want 1", f.replies)
	}
}

func TestRuntimeRoutesDynamicToolCallThroughMCP(t *testing.T) {
	r, f, _ := testRuntime(t)
	defer r.Close(context.Background())
	if err := r.Start(context.Background(), Options{
		DynamicTools: []appserver.DynamicToolNamespace{{Type: "namespace", Name: "toolbox"}},
	}, "test"); err != nil {
		t.Fatal(err)
	}
	f.mcpResult = appserver.MCPToolResult{Content: []json.RawMessage{
		json.RawMessage(`{"type":"text","text":"/workspace"}`),
	}}
	request := appserver.ServerRequest{
		ID:     11,
		Method: "item/tool/call",
		Params: json.RawMessage(`{"threadId":"new-thread","turnId":"turn-1","callId":"call-1","namespace":"toolbox","tool":"pwd","arguments":{}}`),
	}
	if err := r.handleRequest(context.Background(), request, strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	if f.mcpCalls != 1 || f.replies != 1 {
		t.Fatalf("MCP calls=%d replies=%d", f.mcpCalls, f.replies)
	}
	reply, ok := f.lastReply.(map[string]any)
	if !ok || reply["success"] != true {
		t.Fatalf("dynamic tool reply = %#v", f.lastReply)
	}
	content, ok := reply["contentItems"].([]map[string]string)
	if !ok || len(content) != 1 || content[0]["type"] != "inputText" || content[0]["text"] != "/workspace" {
		t.Fatalf("dynamic tool content = %#v", reply["contentItems"])
	}
}

func TestRuntimeRequestHandlerConsumesTerminalInputThroughRun(t *testing.T) {
	r, f, _ := testRuntime(t)
	if err := r.Start(context.Background(), Options{}, "test"); err != nil {
		t.Fatal(err)
	}
	var got string
	handled := make(chan struct{})
	r.SetRequestHandler(func(_ context.Context, request appserver.ServerRequest, input io.Reader) error {
		line, err := bufio.NewReader(input).ReadString('\n')
		if err != nil {
			return err
		}
		got = line
		close(handled)
		return f.ReplyRequest(context.Background(), request.ID, map[string]string{"decision": "accept"})
	})
	inputReader, inputWriter := io.Pipe()
	f.requests <- appserver.ServerRequest{ID: 8, Method: "item/commandExecution/requestApproval"}
	done := make(chan error, 1)
	go func() { done <- r.Run(context.Background(), inputReader, &fakeRenderer{}) }()
	if _, err := io.WriteString(inputWriter, "yes\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("approval handler did not consume terminal input")
	}
	if err := inputWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got != "yes\n" || f.replies != 1 {
		t.Fatalf("approval input=%q replies=%d", got, f.replies)
	}
}

var _ io.Reader = strings.NewReader("")

func testRuntimeWithTimeouts(t *testing.T, timeouts instance.Timeouts) (*Runtime, *fakeClient, *fakeProcess) {
	t.Helper()
	in, err := instance.New(instance.Config{
		Name: "test-timeouts", StateRoot: t.TempDir(), WorkingRoot: t.TempDir(), Timeouts: timeouts,
	})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := in.AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	client, process := newFakeClient(), &fakeProcess{codexVersion: "0.149.0"}
	r, err := New(in, client, process, lock)
	if err != nil {
		t.Fatal(err)
	}
	return r, client, process
}
