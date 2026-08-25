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
	ErrActiveTurn             = errors.New("a turn is already active")
	ErrShuttingDown           = errors.New("runtime is shutting down")
	ErrResumeFailed           = errors.New("primary thread cannot be resumed; start Promptline without 'resume' to create a replacement")
	ErrNoStoredThread         = errors.New("no saved primary thread is available to resume")
	ErrAuthenticationRequired = errors.New("codex authentication required")
	ErrToolboxUnavailable     = errors.New("promptline toolbox MCP server is unavailable")
)

// API is the deliberately small app-server surface used by the terminal.
type API interface {
	Initialize(context.Context, appserver.Initialize) error
	Account(context.Context) (appserver.Account, error)
	StartThread(context.Context, string, string, string) (appserver.Thread, error)
	ResumeThread(context.Context, string, string, string) (appserver.Thread, error)
	StartTurn(context.Context, string, string, string, string) (appserver.Turn, error)
	ListMCPServers(context.Context, string) ([]appserver.MCPServer, error)
	Interrupt(context.Context, string, string) error
	Unsubscribe(context.Context, string) error
}

// Client exposes streams which remain owned by exactly one app-server child.
type Client interface {
	API
	Events() <-chan appserver.Event
	Done() <-chan struct{}
	Err() error
}

type Process interface {
	CodexVersion() string
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
	Delta(string) error
	Text(string) error
	Progress(string) error
	Error(error) error
}

type Options struct {
	Resume   bool
	ResumeID string
}

// Runtime has one selected thread and serializes turns for one instance.
type Runtime struct {
	instance           *instance.Instance
	api                Client
	process            Process
	lock               *instance.Lock
	threadID           string
	mu                 sync.Mutex
	turnID             string
	closing            bool
	closeOnce          sync.Once
	closeErr           error
	requests           requestClient
	handler            RequestHandler
	streamedAgentItems map[string]struct{}
	turnHasOutput      bool
	streamOpen         bool
	turnErrorRendered  bool
	toolboxTools       int
}

func New(in *instance.Instance, api Client, process Process, lock *instance.Lock) (*Runtime, error) {
	if in == nil || api == nil || process == nil || lock == nil {
		return nil, errors.New("runtime requires instance, client, process, and lock")
	}
	r := &Runtime{
		instance:           in,
		api:                api,
		process:            process,
		lock:               lock,
		streamedAgentItems: map[string]struct{}{},
	}
	if requests, ok := api.(requestClient); ok {
		r.requests = requests
	}
	return r, nil
}

// SetRequestHandler installs the sole approval/effect response path. A nil
// handler is safe: every request is declined, never implicitly approved.
func (r *Runtime) SetRequestHandler(handler RequestHandler) { r.handler = handler }

func (r *Runtime) Start(ctx context.Context, opts Options, version string) error {
	ctx, cancel := context.WithTimeout(ctx, r.instance.Timeouts().Startup)
	defer cancel()
	if err := r.api.Initialize(ctx, appserver.Initialize{ClientName: "promptline", ClientVersion: version}); err != nil {
		return fmt.Errorf("initialize app-server: %w", err)
	}
	account, err := r.api.Account(ctx)
	if err != nil {
		return fmt.Errorf("read Codex authentication state: %w", err)
	}
	if !account.Authenticated() {
		return fmt.Errorf(
			"%w for instance %q; run CODEX_HOME=%q %q login, then restart Promptline",
			ErrAuthenticationRequired,
			r.instance.Name(),
			r.instance.CodexHome(),
			r.instance.CodexExecutable(),
		)
	}
	state, err := r.instance.LoadState()
	if err != nil {
		return fmt.Errorf("load instance state: %w", err)
	}
	id := opts.ResumeID
	if id == "" && opts.Resume {
		id = state.LastPrimaryThreadID
		if id == "" {
			return ErrNoStoredThread
		}
	}
	var thread appserver.Thread
	developerInstructions := initPrompt()
	if id != "" {
		thread, err = r.api.ResumeThread(ctx, id, r.instance.Model(), developerInstructions)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrResumeFailed, err)
		}
	} else {
		thread, err = r.api.StartThread(
			ctx,
			r.instance.WorkingDirectory(),
			r.instance.Model(),
			developerInstructions,
		)
		if err != nil {
			return fmt.Errorf("start primary thread: %w", err)
		}
	}
	if thread.ID == "" {
		return errors.New("app-server returned a thread without an ID")
	}
	r.mu.Lock()
	r.threadID = thread.ID
	r.mu.Unlock()
	if r.instance.ToolboxEnabled() {
		toolCount, err := r.requireToolbox(ctx, thread.ID)
		if err != nil {
			return err
		}
		r.toolboxTools = toolCount
	}
	state.LastPrimaryThreadID = thread.ID
	state.CodexVersion = r.process.CodexVersion()
	if err := r.instance.SaveState(state); err != nil {
		return fmt.Errorf("persist primary thread: %w", err)
	}
	return nil
}

func (r *Runtime) requireToolbox(ctx context.Context, threadID string) (int, error) {
	servers, err := r.api.ListMCPServers(ctx, threadID)
	if err != nil {
		return 0, fmt.Errorf("%w: inspect MCP servers: %v", ErrToolboxUnavailable, err)
	}
	for _, server := range servers {
		if server.Name != "promptline-toolbox" {
			continue
		}
		for _, required := range []string{"ls", "pwd", "cat"} {
			if _, ok := server.Tools[required]; !ok {
				return 0, fmt.Errorf("%w: server is missing required tool %q", ErrToolboxUnavailable, required)
			}
		}
		return len(server.Tools), nil
	}
	return 0, fmt.Errorf("%w: Codex did not load promptline-toolbox from its instance config", ErrToolboxUnavailable)
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
	turn, err := r.api.StartTurn(ctx, threadID, text, "", r.instance.Model())
	if err != nil {
		return appserver.Turn{}, fmt.Errorf("start turn: %w", err)
	}
	if turn.ID == "" {
		return appserver.Turn{}, errors.New("app-server returned a turn without an ID")
	}
	r.mu.Lock()
	r.turnID = turn.ID
	r.streamedAgentItems = map[string]struct{}{}
	r.turnHasOutput = false
	r.streamOpen = false
	r.turnErrorRendered = false
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
	if r.toolboxTools > 0 {
		if err := render.Progress(fmt.Sprintf("toolbox ready: %d tools", r.toolboxTools)); err != nil {
			return err
		}
	}
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
			} else if err := render.Progress("working"); err != nil {
				return err
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
		return false, render.Progress(fmt.Sprintf(
			"instance=%s thread=%s model=%s toolbox-tools=%d",
			r.instance.Name(), r.ThreadID(), r.instance.Model(), r.toolboxTools,
		))
	default:
		if strings.HasPrefix(line, "/") {
			return false, render.Error(fmt.Errorf("unknown command %q", line))
		}
		return false, nil
	}
}

func (r *Runtime) renderEvent(event appserver.Event, render Renderer) error {
	switch event.Method {
	case "item/agentMessage/delta":
		return r.renderAgentMessageDelta(event.Params, render)
	case "turn/completed":
		return r.renderTurnCompletion(event.Params, render)
	case "error":
		return r.renderErrorEvent(event.Params, render)
	}
	item, err := appserver.DecodeItem(event.Params)
	if err != nil {
		return nil
	} // additive notifications are not terminal UI errors
	if item.Type == "agentMessage" || item.Type == "text" {
		r.mu.Lock()
		_, streamed := r.streamedAgentItems[item.ID]
		if !streamed && item.Text != "" {
			r.turnHasOutput = true
		}
		r.mu.Unlock()
		if streamed || item.Text == "" {
			return nil
		}
		return render.Text(item.Text)
	}
	isProgressItem := item.Type == "commandExecution" || item.Type == "fileChange" ||
		item.Type == "mcpToolCall" || item.Type == "webSearch"
	if event.Method == "item/started" && isProgressItem {
		return render.Progress(item.Type)
	}
	return nil
}

func (r *Runtime) renderAgentMessageDelta(params []byte, render Renderer) error {
	delta, err := appserver.DecodeAgentMessageDelta(params)
	if err != nil || delta.Delta == "" {
		return nil
	}
	r.mu.Lock()
	r.streamedAgentItems[delta.ItemID] = struct{}{}
	r.turnHasOutput = true
	r.streamOpen = true
	r.mu.Unlock()
	return render.Delta(delta.Delta)
}

func (r *Runtime) renderTurnCompletion(params []byte, render Renderer) error {
	completion, _ := appserver.DecodeTurnCompletion(params)
	r.mu.Lock()
	turnID := r.turnID
	streamOpen := r.streamOpen
	hasOutput := r.turnHasOutput
	errorRendered := r.turnErrorRendered
	r.streamOpen = false
	r.mu.Unlock()
	if streamOpen {
		if err := render.Text(""); err != nil {
			return err
		}
	}
	if !hasOutput && completion.FinalMessage.Text != "" {
		if err := render.Text(completion.FinalMessage.Text); err != nil {
			return err
		}
	}
	if completion.Status == "failed" && completion.ErrorMessage != "" && !errorRendered {
		if err := render.Error(errors.New(completion.ErrorMessage)); err != nil {
			return err
		}
	}
	r.completeTurn(turnID)
	return render.Prompt()
}

func (r *Runtime) renderErrorEvent(params []byte, render Renderer) error {
	message, err := appserver.DecodeErrorMessage(params)
	if err != nil {
		return nil
	}
	r.mu.Lock()
	streamOpen := r.streamOpen
	r.streamOpen = false
	r.turnErrorRendered = true
	r.mu.Unlock()
	if streamOpen {
		if err := render.Text(""); err != nil {
			return err
		}
	}
	return render.Error(errors.New(message))
}

func (r *Runtime) Close(ctx context.Context) error {
	r.closeOnce.Do(func() {
		closeCtx, cancel := context.WithTimeout(ctx, r.instance.Timeouts().Shutdown)
		defer cancel()
		r.mu.Lock()
		r.closing = true
		threadID := r.threadID
		r.mu.Unlock()
		var errs []error
		if threadID != "" {
			if err := r.api.Unsubscribe(closeCtx, threadID); err != nil {
				errs = append(errs, fmt.Errorf("unsubscribe primary thread: %w", err))
			}
		}
		if err := r.process.Close(closeCtx); err != nil {
			errs = append(errs, err)
		}
		if err := r.lock.Close(); err != nil {
			errs = append(errs, err)
		}
		r.closeErr = errors.Join(errs...)
	})
	return r.closeErr
}
