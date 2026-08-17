package runtime

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"promptline/internal/appserver"
	"promptline/internal/instance"
)

type fakeClient struct {
	events                                   chan appserver.Event
	done                                     chan struct{}
	startThreads, resumes, turns, interrupts int
	resumeErr                                error
}

func newFakeClient() *fakeClient {
	return &fakeClient{events: make(chan appserver.Event, 8), done: make(chan struct{})}
}
func (f *fakeClient) Initialize(context.Context, appserver.Initialize) error { return nil }
func (f *fakeClient) Account(context.Context) ([]byte, error)                { return []byte(`{}`), nil }
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
func (f *fakeClient) Done() <-chan struct{}                           { return f.done }
func (f *fakeClient) Err() error                                      { return nil }

type fakeProcess struct{ closed int }

func (p *fakeProcess) Close(context.Context) error { p.closed++; return nil }

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

var _ io.Reader = strings.NewReader("")
