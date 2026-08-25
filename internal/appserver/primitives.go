package appserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type Initialize struct {
	ClientName, ClientVersion string
	Experimental              bool
}
type Thread struct {
	ID string `json:"id"`
}
type Turn struct {
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
}
type Item struct {
	ID, ThreadID, TurnID, Type string
	Text                       string
	Raw                        json.RawMessage
}
type API struct {
	c           *Client
	mu          sync.Mutex
	initialized bool
	replied     map[uint64]struct{}
}

func NewAPI(c *Client) *API { return &API{c: c, replied: make(map[uint64]struct{})} }
func (a *API) Initialize(ctx context.Context, in Initialize) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.initialized {
		return errors.New("app-server already initialized")
	}
	if in.Experimental {
		return errors.New("experimental api is disabled")
	}
	var r struct{}
	if err := a.call(ctx, "initialize", map[string]any{"clientInfo": map[string]string{"name": in.ClientName, "version": in.ClientVersion}, "capabilities": map[string]any{"experimentalApi": false}}, &r, true); err != nil {
		return err
	}
	if err := a.c.Notify("initialized", map[string]any{}); err != nil {
		return err
	}
	a.initialized = true
	return nil
}
func (a *API) require() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.initialized {
		return ErrNotInitialized
	}
	return nil
}
func (a *API) call(ctx context.Context, method string, p any, out any, read bool) error {
	b, err := a.c.Call(ctx, method, p, read)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode %s: %w", method, err)
	}
	return nil
}
func (a *API) Account(ctx context.Context) (json.RawMessage, error) {
	if err := a.require(); err != nil {
		return nil, err
	}
	return a.c.Call(ctx, "account/read", map[string]bool{"refreshToken": false}, true)
}
func (a *API) StartThread(ctx context.Context, cwd, model string) (Thread, error) {
	if err := a.require(); err != nil {
		return Thread{}, err
	}
	var r struct {
		Thread Thread `json:"thread"`
	}
	err := a.call(ctx, "thread/start", map[string]string{"cwd": cwd, "model": model}, &r, false)
	return r.Thread, err
}
func (a *API) ResumeThread(ctx context.Context, id string) (Thread, error) {
	if err := a.require(); err != nil {
		return Thread{}, err
	}
	var r struct {
		Thread Thread `json:"thread"`
	}
	err := a.call(ctx, "thread/resume", map[string]string{"threadId": id}, &r, false)
	return r.Thread, err
}
func (a *API) ReadThread(ctx context.Context, id string) (Thread, error) {
	if err := a.require(); err != nil {
		return Thread{}, err
	}
	var r struct {
		Thread Thread `json:"thread"`
	}
	err := a.call(ctx, "thread/read", map[string]string{"threadId": id}, &r, true)
	return r.Thread, err
}
func (a *API) StartTurn(ctx context.Context, threadID, text, clientMessageID string) (Turn, error) {
	if err := a.require(); err != nil {
		return Turn{}, err
	}
	var r struct {
		Turn Turn `json:"turn"`
	}
	p := map[string]any{"threadId": threadID, "input": []map[string]string{{"type": "text", "text": text}}}
	if clientMessageID != "" {
		p["clientUserMessageId"] = clientMessageID
	}
	err := a.call(ctx, "turn/start", p, &r, false)
	return r.Turn, err
}
func (a *API) Interrupt(ctx context.Context, threadID, turnID string) error {
	if err := a.require(); err != nil {
		return err
	}
	return a.call(ctx, "turn/interrupt", map[string]string{"threadId": threadID, "turnId": turnID}, nil, false)
}

// ReplyRequest sends at most one decision for a server-initiated approval or
// user-input request. Policy belongs to the caller, never this transport.
func (a *API) ReplyRequest(ctx context.Context, id uint64, decision any) error {
	a.mu.Lock()
	if _, exists := a.replied[id]; exists {
		a.mu.Unlock()
		return errors.New("server request already answered")
	}
	a.replied[id] = struct{}{}
	a.mu.Unlock()
	if err := a.c.Reply(ctx, id, decision, nil); err != nil {
		return err
	}
	return nil
}

// DecodeItem extracts the stable identity fields while retaining additive
// item fields for callers that need the version-specific details.
func DecodeItem(params json.RawMessage) (Item, error) {
	var wire struct {
		Item struct {
			ID       string `json:"id"`
			ThreadID string `json:"threadId"`
			TurnID   string `json:"turnId"`
			Type     string `json:"type"`
			Text     string `json:"text"`
		} `json:"item"`
		ThreadID string `json:"threadId"`
		TurnID   string `json:"turnId"`
	}
	if err := json.Unmarshal(params, &wire); err != nil {
		return Item{}, err
	}
	if wire.Item.ID == "" || wire.Item.Type == "" {
		return Item{}, errors.New("item event is missing required item identity")
	}
	threadID, turnID := wire.Item.ThreadID, wire.Item.TurnID
	if threadID == "" {
		threadID = wire.ThreadID
	}
	if turnID == "" {
		turnID = wire.TurnID
	}
	return Item{ID: wire.Item.ID, ThreadID: threadID, TurnID: turnID, Type: wire.Item.Type, Text: wire.Item.Text, Raw: append(json.RawMessage(nil), params...)}, nil
}
