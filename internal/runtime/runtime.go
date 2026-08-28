// Package runtime composes one Promptline instance with one app-server client.
package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"promptline/internal/appserver"
	"promptline/internal/governance"
	"promptline/internal/instance"
)

var (
	ErrActiveTurn             = errors.New("a turn is already active")
	ErrShuttingDown           = errors.New("runtime is shutting down")
	ErrResumeFailed           = errors.New("primary thread cannot be resumed; start Promptline without 'resume' to create a replacement")
	ErrNoStoredThread         = errors.New("no saved primary thread is available to resume")
	ErrAuthenticationRequired = errors.New("codex authentication required")
	ErrToolboxUnavailable     = errors.New("promptline toolbox MCP server is unavailable")
	errNoPrimaryThread        = errors.New("no primary thread selected")
)

// API is the deliberately small app-server surface used by the terminal.
type API interface {
	Initialize(context.Context, appserver.Initialize) error
	Account(context.Context) (appserver.Account, error)
	StartThread(
		context.Context, string, string, string, []appserver.DynamicToolNamespace,
	) (appserver.Thread, error)
	ResumeThread(
		context.Context, string, string, string, []appserver.DynamicToolNamespace,
	) (appserver.Thread, error)
	StartTurn(context.Context, string, string, string, string) (appserver.Turn, error)
	ListMCPServers(context.Context, string) ([]appserver.MCPServer, error)
	CallMCPTool(context.Context, string, string, string, json.RawMessage) (appserver.MCPToolResult, error)
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

// EventKind identifies a lifecycle fact independently from terminal rendering.
type EventKind string

const (
	EventTurnAccepted      EventKind = "turn_accepted"
	EventTurnCompleted     EventKind = "turn_completed"
	EventTurnFailed        EventKind = "turn_failed"
	EventToolActivity      EventKind = "tool_activity"
	EventApprovalRequested EventKind = "approval_requested"
	EventApprovalResolved  EventKind = "approval_resolved"
	EventShutdown          EventKind = "shutdown"
)

// Event is a semantic runtime observation. Its fields deliberately avoid
// terminal text and formatting so callers can observe behavior without making
// the console part of the runtime contract.
type Event struct {
	Kind     EventKind
	ThreadID string
	TurnID   string
	Tool     string
	Err      error
}

// Observer receives runtime lifecycle observations. Implementations must not
// block; runtime execution remains the owner of ordering and delivery.
type Observer interface{ Observe(Event) }

type Options struct {
	Resume       bool
	ResumeID     string
	DynamicTools []appserver.DynamicToolNamespace
}

// Runtime has one selected thread and serializes turns for one instance.
type Runtime struct {
	instance     *instance.Instance
	api          Client
	process      Process
	lock         *instance.Lock
	closeOnce    sync.Once
	closeErr     error
	requests     requestClient
	handler      RequestHandler
	state        runtimeState
	toolboxTools int
	observer     Observer
}

func New(in *instance.Instance, api Client, process Process, lock *instance.Lock) (*Runtime, error) {
	if in == nil || api == nil || process == nil || lock == nil {
		return nil, errors.New("runtime requires instance, client, process, and lock")
	}
	r := &Runtime{
		instance: in,
		api:      api,
		process:  process,
		lock:     lock,
		state:    newRuntimeState(),
	}
	if requests, ok := api.(requestClient); ok {
		r.requests = requests
	}
	return r, nil
}

// SetRequestHandler installs the sole approval/effect response path. A nil
// handler is safe: every request is declined, never implicitly approved.
func (r *Runtime) SetRequestHandler(handler RequestHandler) { r.handler = handler }

// ActiveTurnIdentity is a synchronized snapshot for approval correlation.
func (r *Runtime) ActiveTurnIdentity() (string, string) { return r.state.activeTurn() }
func (r *Runtime) ApprovalIdentity(request appserver.ServerRequest) governance.ApprovalIdentity {
	var wire struct {
		ItemID        string `json:"itemId"`
		EnvironmentID string `json:"environmentId"`
		ApprovalID    string `json:"approvalId"`
	}
	_ = json.Unmarshal(request.Params, &wire)
	threadID, turnID := r.state.activeTurn()
	return governance.ApprovalIdentity{ThreadID: threadID, TurnID: turnID, ItemID: wire.ItemID, EnvironmentID: wire.EnvironmentID, ApprovalID: wire.ApprovalID, PendingItem: r.state.pendingItem(wire.ItemID).Raw}
}

// SetObserver installs an optional semantic observer. A nil observer restores
// the production default, which performs no observation.
func (r *Runtime) SetObserver(observer Observer) { r.observer = observer }

func (r *Runtime) observe(event Event) {
	if r.observer != nil {
		r.observer.Observe(event)
	}
}

func (r *Runtime) Start(ctx context.Context, opts Options, version string) error {
	ctx, cancel := context.WithTimeout(ctx, r.instance.Timeouts().Startup)
	defer cancel()
	initialize := appserver.Initialize{
		ClientName:    "promptline",
		ClientVersion: version,
		Experimental:  len(opts.DynamicTools) > 0,
	}
	if err := r.api.Initialize(ctx, initialize); err != nil {
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
	developerInstructions, err := initPrompt()
	if err != nil {
		return err
	}
	if id != "" {
		thread, err = r.api.ResumeThread(ctx, id, r.instance.Model(), developerInstructions, opts.DynamicTools)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrResumeFailed, err)
		}
	} else {
		thread, err = r.api.StartThread(
			ctx,
			r.instance.WorkingDirectory(),
			r.instance.Model(),
			developerInstructions,
			opts.DynamicTools,
		)
		if err != nil {
			return fmt.Errorf("start primary thread: %w", err)
		}
	}
	if thread.ID == "" {
		return errors.New("app-server returned a thread without an ID")
	}
	r.state.setThread(thread.ID)
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

func (r *Runtime) ThreadID() string { return r.state.thread() }

func (r *Runtime) HasActiveTurn() bool { return r.state.hasActiveTurn() }

func (r *Runtime) StartTurn(ctx context.Context, text string) (appserver.Turn, error) {
	threadID, err := r.state.beginTurn()
	if err != nil {
		return appserver.Turn{}, err
	}
	turn, err := r.api.StartTurn(ctx, threadID, text, "", r.instance.Model())
	if err != nil {
		r.state.rejectTurn()
		return appserver.Turn{}, fmt.Errorf("start turn: %w", err)
	}
	if turn.ID == "" {
		r.state.rejectTurn()
		return appserver.Turn{}, errors.New("app-server returned a turn without an ID")
	}
	r.state.acceptTurn(turn.ID)
	r.observe(Event{Kind: EventTurnAccepted, ThreadID: threadID, TurnID: turn.ID})
	return turn, nil
}

func (r *Runtime) Interrupt(ctx context.Context) error {
	threadID, turnID := r.state.activeTurn()
	if turnID == "" {
		return nil
	}
	return r.api.Interrupt(ctx, threadID, turnID)
}

func (r *Runtime) completeTurn(turnID string) bool { return r.state.completeTurn(turnID) }

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
		// A server request already waiting in the queue owns the next terminal
		// line. Handle it before ordinary prompt input so an approval answer
		// cannot be consumed as a new user turn.
		select {
		case request := <-r.requestStream():
			if err := r.handleRequest(ctx, request, &lineReader{lines: lines}); err != nil {
				return err
			}
			continue
		default:
		}
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
	if request.Method == "item/tool/call" {
		return r.handleDynamicToolCall(ctx, request)
	}
	r.observe(Event{Kind: EventApprovalRequested, ThreadID: r.ThreadID()})
	if r.handler != nil {
		if err := r.handler(ctx, request, input); err != nil {
			r.observe(Event{Kind: EventApprovalResolved, ThreadID: r.ThreadID(), Err: err})
			return err
		}
		r.observe(Event{Kind: EventApprovalResolved, ThreadID: r.ThreadID()})
		return nil
	}
	err := r.requests.ReplyRequest(ctx, request.ID, map[string]string{"decision": "decline"})
	r.observe(Event{Kind: EventApprovalResolved, ThreadID: r.ThreadID(), Err: err})
	return err
}

func (r *Runtime) handleDynamicToolCall(ctx context.Context, request appserver.ServerRequest) error {
	var call struct {
		ThreadID  string          `json:"threadId"`
		Namespace string          `json:"namespace"`
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(request.Params, &call); err != nil {
		return r.replyDynamicToolError(ctx, request.ID, "invalid toolbox call: "+err.Error())
	}
	threadID := r.state.thread()
	if call.Namespace != "toolbox" || call.Tool == "" || call.ThreadID != threadID {
		return r.replyDynamicToolError(ctx, request.ID, "invalid toolbox namespace, tool, or thread")
	}
	r.observe(Event{Kind: EventToolActivity, ThreadID: threadID, Tool: call.Tool})
	result, err := r.api.CallMCPTool(ctx, threadID, "promptline-toolbox", call.Tool, call.Arguments)
	if err != nil {
		return r.replyDynamicToolError(ctx, request.ID, err.Error())
	}
	contentItems := make([]map[string]string, 0, len(result.Content))
	for _, raw := range result.Content {
		var content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &content); err == nil && content.Type == "text" {
			contentItems = append(contentItems, map[string]string{"type": "inputText", "text": content.Text})
			continue
		}
		contentItems = append(contentItems, map[string]string{"type": "inputText", "text": string(raw)})
	}
	return r.requests.ReplyRequest(ctx, request.ID, map[string]any{
		"contentItems": contentItems,
		"success":      !result.IsError,
	})
}

func (r *Runtime) replyDynamicToolError(ctx context.Context, id uint64, message string) error {
	return r.requests.ReplyRequest(ctx, id, map[string]any{
		"contentItems": []map[string]string{{"type": "inputText", "text": message}},
		"success":      false,
	})
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
		if r.state.reduceItem(item.ID, item.Text) {
			return nil
		}
		return render.Text(item.Text)
	}
	if event.Method == "item/started" && (item.Type == "fileChange" || item.Type == "commandExecution") {
		r.state.rememberPending(item)
	}
	isProgressItem := item.Type == "commandExecution" || item.Type == "fileChange" ||
		item.Type == "mcpToolCall" || item.Type == "dynamicToolCall" || item.Type == "webSearch"
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
	r.state.markDelta(delta.ItemID)
	return render.Delta(delta.Delta)
}

func (r *Runtime) renderTurnCompletion(params []byte, render Renderer) error {
	completion, _ := appserver.DecodeTurnCompletion(params)
	turnID, streamOpen, hasOutput, errorRendered := r.state.beginCompletion()
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
	if r.completeTurn(turnID) {
		kind := EventTurnCompleted
		if completion.Status == "failed" {
			kind = EventTurnFailed
		}
		r.observe(Event{Kind: kind, ThreadID: r.ThreadID(), TurnID: turnID})
	}
	return render.Prompt()
}

func (r *Runtime) renderErrorEvent(params []byte, render Renderer) error {
	message, err := appserver.DecodeErrorMessage(params)
	if err != nil {
		return nil
	}
	streamOpen := r.state.beginError()
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
		threadID := r.state.beginShutdown()
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
		r.observe(Event{Kind: EventShutdown, ThreadID: threadID, Err: r.closeErr})
	})
	return r.closeErr
}
