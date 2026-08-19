// Package runtime composes one Promptline instance with one app-server client.
package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"promptline/internal/appserver"
	"promptline/internal/instance"
)

var (
	ErrActiveTurn   = errors.New("a turn is already active")
	ErrShuttingDown = errors.New("runtime is shutting down")
	ErrResumeFailed = errors.New("stored primary thread cannot be resumed; rerun with --new to create a replacement")
)

// API is the deliberately small app-server surface used by the terminal.
type API interface {
	Initialize(context.Context, appserver.Initialize) error
	Account(context.Context) ([]byte, error)
	StartThread(context.Context, string, string) (appserver.Thread, error)
	ResumeThread(context.Context, string) (appserver.Thread, error)
	StartTurn(context.Context, string, string, string) (appserver.Turn, error)
	Interrupt(context.Context, string, string) error
}

// Client exposes streams which remain owned by exactly one app-server child.
type Client interface {
	API
	Events() <-chan appserver.Event
	Done() <-chan struct{}
	Err() error
}

type Process interface {
	Close(context.Context) error
}

// RequestHandler answers one server-initiated effect request. It is kept out
// of the transport package so policy and terminal concerns remain injectable.
type RequestHandler func(context.Context, appserver.ServerRequest, io.Reader) error

type requestClient interface {
	Requests() <-chan appserver.ServerRequest
	ReplyRequest(context.Context, uint64, any) error
}

type Renderer interface {
	Prompt() error
	Text(string) error
	Progress(string) error
	Error(error) error
}

type Options struct {
	New      bool
	ResumeID string
}

// Runtime has one selected thread and serializes turns for one instance.
type Runtime struct {
	instance  *instance.Instance
	api       Client
	process   Process
	lock      *instance.Lock
	threadID  string
	mu        sync.Mutex
	turnID    string
	closing   bool
	closeOnce sync.Once
	closeErr  error
	requests  requestClient
	handler   RequestHandler
}

func New(in *instance.Instance, api Client, process Process, lock *instance.Lock) (*Runtime, error) {
	if in == nil || api == nil || process == nil || lock == nil {
		return nil, errors.New("runtime requires instance, client, process, and lock")
	}
	r := &Runtime{instance: in, api: api, process: process, lock: lock}
	if requests, ok := api.(requestClient); ok {
		r.requests = requests
	}
	return r, nil
}

// SetRequestHandler installs the sole approval/effect response path. A nil
// handler is safe: every request is declined, never implicitly approved.
func (r *Runtime) SetRequestHandler(handler RequestHandler) { r.handler = handler }

func (r *Runtime) Start(ctx context.Context, opts Options, version string) error {
	if err := r.api.Initialize(ctx, appserver.Initialize{ClientName: "promptline", ClientVersion: version}); err != nil {
		return fmt.Errorf("initialize app-server: %w", err)
	}
	if _, err := r.api.Account(ctx); err != nil {
		return fmt.Errorf("read Codex authentication state: %w", err)
	}
	state, err := r.instance.LoadState()
	if err != nil {
		return fmt.Errorf("load instance state: %w", err)
	}
	id := opts.ResumeID
	if id == "" && !opts.New {
		id = state.LastPrimaryThreadID
	}
	var thread appserver.Thread
	if id != "" {
		thread, err = r.api.ResumeThread(ctx, id)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrResumeFailed, err)
		}
	} else {
		thread, err = r.api.StartThread(ctx, r.instance.WorkingDirectory(), r.instance.Model())
		if err != nil {
			return fmt.Errorf("start primary thread: %w", err)
		}
	}
	if thread.ID == "" {
		return errors.New("app-server returned a thread without an ID")
	}
	state.LastPrimaryThreadID = thread.ID
	if err := r.instance.SaveState(state); err != nil {
		return fmt.Errorf("persist primary thread: %w", err)
	}
	r.mu.Lock()
	r.threadID = thread.ID
	r.mu.Unlock()
	return nil
}

func (r *Runtime) ThreadID() string { r.mu.Lock(); defer r.mu.Unlock(); return r.threadID }

func (r *Runtime) HasActiveTurn() bool { r.mu.Lock(); defer r.mu.Unlock(); return r.turnID != "" }

func (r *Runtime) StartTurn(ctx context.Context, text string) (appserver.Turn, error) {
	r.mu.Lock()
	if r.closing {
		r.mu.Unlock()
		return appserver.Turn{}, ErrShuttingDown
	}
	if r.turnID != "" {
		r.mu.Unlock()
		return appserver.Turn{}, ErrActiveTurn
	}
	threadID := r.threadID
	r.mu.Unlock()
	if threadID == "" {
		return appserver.Turn{}, errors.New("no primary thread selected")
	}
	turn, err := r.api.StartTurn(ctx, threadID, text, "")
	if err != nil {
		return appserver.Turn{}, fmt.Errorf("start turn: %w", err)
	}
	if turn.ID == "" {
		return appserver.Turn{}, errors.New("app-server returned a turn without an ID")
	}
	r.mu.Lock()
	r.turnID = turn.ID
	r.mu.Unlock()
	return turn, nil
}

func (r *Runtime) Interrupt(ctx context.Context) error {
	r.mu.Lock()
	threadID, turnID := r.threadID, r.turnID
	r.mu.Unlock()
	if turnID == "" {
		return nil
	}
	return r.api.Interrupt(ctx, threadID, turnID)
}

func (r *Runtime) completeTurn(turnID string) {
	r.mu.Lock()
	if r.turnID == turnID {
		r.turnID = ""
	}
	r.mu.Unlock()
}

// Run reads one foreground terminal stream. It never reads or writes protocol stdout.
func (r *Runtime) Run(ctx context.Context, input io.Reader, render Renderer) error {
	lines := make(chan string)
	errs := make(chan error, 1)
	go func() {
		s := bufio.NewScanner(input)
		s.Buffer(make([]byte, 4096), 1<<20)
		for s.Scan() {
			lines <- s.Text()
		}
		errs <- s.Err()
		close(lines)
	}()
	if err := render.Prompt(); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			_ = r.Interrupt(context.Background())
			return r.Close(context.Background())
		case <-r.api.Done():
			return fmt.Errorf("app-server exited: %w", r.api.Err())
		case err := <-errs:
			if err != nil {
				return err
			}
			return r.Close(context.Background())
		case line, ok := <-lines:
			if !ok {
				return r.Close(context.Background())
			}
			if quit, err := r.command(ctx, strings.TrimSpace(line), render); err != nil {
				return err
			} else if quit {
				return r.Close(context.Background())
			}
			if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "/") {
				continue
			}
			if _, err := r.StartTurn(ctx, line); err != nil {
				if e := render.Error(err); e != nil {
					return e
				}
			}
		case event := <-r.api.Events():
			if err := r.renderEvent(event, render); err != nil {
				return err
			}
		case request := <-r.requestStream():
			if err := r.handleRequest(ctx, request, &lineReader{lines: lines}); err != nil {
				return err
			}
		}
	}
}

func (r *Runtime) requestStream() <-chan appserver.ServerRequest {
	if r.requests == nil {
		return nil
	}
	return r.requests.Requests()
}

func (r *Runtime) handleRequest(ctx context.Context, request appserver.ServerRequest, input io.Reader) error {
	if r.requests == nil {
		return nil
	}
	if r.handler != nil {
		if err := r.handler(ctx, request, input); err != nil {
			return err
		}
		return nil
	}
	return r.requests.ReplyRequest(ctx, request.ID, map[string]string{"decision": "decline"})
}

// lineReader is the sole consumer of terminal input while an approval is
// pending. The scanner in Run remains the only reader of the underlying
// stream, preventing approval responses from racing regular turn input.
type lineReader struct {
	lines   <-chan string
	pending []byte
}

func (r *lineReader) Read(p []byte) (int, error) {
	for len(r.pending) == 0 {
		line, ok := <-r.lines
		if !ok {
			return 0, io.EOF
		}
		r.pending = append([]byte(line), '\n')
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (r *Runtime) command(ctx context.Context, line string, render Renderer) (bool, error) {
	switch line {
	case "", "/help":
		if line == "/help" {
			return false, render.Progress("commands: /help /interrupt /info /quit")
		}
		return false, nil
	case "/quit", "/exit":
		return true, nil
	case "/interrupt":
		return false, r.Interrupt(ctx)
	case "/info":
		return false, render.Progress("instance=" + r.instance.Name() + " thread=" + r.ThreadID())
	default:
		if strings.HasPrefix(line, "/") {
			return false, render.Error(fmt.Errorf("unknown command %q", line))
		}
		return false, nil
	}
}

func (r *Runtime) renderEvent(event appserver.Event, render Renderer) error {
	if event.Method == "turn/completed" {
		r.mu.Lock()
		id := r.turnID
		r.mu.Unlock()
		r.completeTurn(id)
		return render.Prompt()
	}
	item, err := appserver.DecodeItem(event.Params)
	if err != nil {
		return nil
	} // additive notifications are not terminal UI errors
	if item.Type == "agentMessage" || item.Type == "text" {
		return render.Text(item.Text)
	}
	if item.Type == "commandExecution" || item.Type == "fileChange" {
		return render.Progress(item.Type)
	}
	return nil
}

func (r *Runtime) Close(ctx context.Context) error {
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closing = true
		r.mu.Unlock()
		var errs []error
		if err := r.process.Close(ctx); err != nil {
			errs = append(errs, err)
		}
		if err := r.lock.Close(); err != nil {
			errs = append(errs, err)
		}
		r.closeErr = errors.Join(errs...)
	})
	return r.closeErr
}
