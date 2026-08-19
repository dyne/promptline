package runtime

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"promptline/internal/appserver"
	"promptline/internal/instance"
)

type fakeClient struct {
	events                                            chan appserver.Event
	requests                                          chan appserver.ServerRequest
	done                                              chan struct{}
	startThreads, resumes, turns, interrupts, replies int
	resumeErr                                         error
	initializeWait                                    bool
}

func newFakeClient() *fakeClient {
	return &fakeClient{events: make(chan appserver.Event, 8), requests: make(chan appserver.ServerRequest, 8), done: make(chan struct{})}
}
func (f *fakeClient) Initialize(ctx context.Context, _ appserver.Initialize) error {
	if f.initializeWait {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}
func (f *fakeClient) Account(context.Context) ([]byte, error) { return []byte(`{}`), nil }
func (f *fakeClient) StartThread(context.Context, string, string) (appserver.Thread, error) {
	f.startThreads++
	return appserver.Thread{ID: "new-thread"}, nil
}
func (f *fakeClient) ResumeThread(context.Context, string) (appserver.Thread, error) {
	f.resumes++
	if f.resumeErr != nil {
		return appserver.Thread{}, f.resumeErr
	}
	return appserver.Thread{ID: "stored-thread"}, nil
}
func (f *fakeClient) StartTurn(context.Context, string, string, string) (appserver.Turn, error) {
	f.turns++
	return appserver.Turn{ID: "turn-1"}, nil
}
func (f *fakeClient) Interrupt(context.Context, string, string) error { f.interrupts++; return nil }
func (f *fakeClient) Events() <-chan appserver.Event                  { return f.events }
func (f *fakeClient) Requests() <-chan appserver.ServerRequest        { return f.requests }
func (f *fakeClient) ReplyRequest(context.Context, uint64, any) error { f.replies++; return nil }
func (f *fakeClient) Done() <-chan struct{}                           { return f.done }
func (f *fakeClient) Err() error                                      { return nil }

type fakeProcess struct{ closed int }

func (p *fakeProcess) Close(context.Context) error { p.closed++; return nil }

type blockingProcess struct{}

func (*blockingProcess) Close(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

type fakeRenderer struct{ strings.Builder }

func (r *fakeRenderer) Prompt() error           { _, e := r.WriteString("> "); return e }
func (r *fakeRenderer) Text(s string) error     { _, e := r.WriteString(s); return e }
func (r *fakeRenderer) Progress(s string) error { _, e := r.WriteString(s); return e }
func (r *fakeRenderer) Error(e error) error     { _, e2 := r.WriteString(e.Error()); return e2 }

func testRuntime(t *testing.T) (*Runtime, *fakeClient, *fakeProcess) {
	t.Helper()
	in, err := instance.New(instance.Config{Name: "test", StateRoot: t.TempDir(), WorkingRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := in.AcquireLock()
	if err != nil {
		t.Fatal(err)
	}
	f, p := newFakeClient(), &fakeProcess{}
	r, err := New(in, f, p, lock)
	if err != nil {
		t.Fatal(err)
	}
	return r, f, p
}

func TestRuntime_StartSelectsOneThread(t *testing.T) {
	r, f, _ := testRuntime(t)
	defer r.Close(context.Background())
	if err := r.Start(context.Background(), Options{}, "test"); err != nil {
		t.Fatal(err)
	}
	if f.startThreads != 1 || r.ThreadID() != "new-thread" {
		t.Fatalf("start=%d thread=%q", f.startThreads, r.ThreadID())
	}
	if _, err := r.StartTurn(context.Background(), "hello"); err != nil {
		t.Fatal(err)
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

func TestRuntime_StoredThreadResumeFailureDoesNotCreateThread(t *testing.T) {
	r, f, _ := testRuntime(t)
	defer r.Close(context.Background())
	if err := r.instance.SaveState(instance.State{LastPrimaryThreadID: "old"}); err != nil {
		t.Fatal(err)
	}
	f.resumeErr = errors.New("missing")
	err := r.Start(context.Background(), Options{}, "test")
	if !errors.Is(err, ErrResumeFailed) || f.startThreads != 0 {
		t.Fatalf("err=%v start=%d", err, f.startThreads)
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
	if f.turns != 1 || p.closed != 1 {
		t.Fatalf("turns=%d closed=%d", f.turns, p.closed)
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

func TestRuntimeRequestHandlerConsumesTerminalInputThroughRun(t *testing.T) {
	r, f, _ := testRuntime(t)
	if err := r.Start(context.Background(), Options{}, "test"); err != nil {
		t.Fatal(err)
	}
	var got string
	r.SetRequestHandler(func(_ context.Context, request appserver.ServerRequest, input io.Reader) error {
		line, err := bufio.NewReader(input).ReadString('\n')
		if err != nil {
			return err
		}
		got = line
		return f.ReplyRequest(context.Background(), request.ID, map[string]string{"decision": "accept"})
	})
	inputReader, inputWriter := io.Pipe()
	f.requests <- appserver.ServerRequest{ID: 8, Method: "item/commandExecution/requestApproval"}
	done := make(chan error, 1)
	go func() { done <- r.Run(context.Background(), inputReader, &fakeRenderer{}) }()
	if _, err := io.WriteString(inputWriter, "yes\n"); err != nil {
		t.Fatal(err)
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
	client, process := newFakeClient(), &fakeProcess{}
	r, err := New(in, client, process, lock)
	if err != nil {
		t.Fatal(err)
	}
	return r, client, process
}
