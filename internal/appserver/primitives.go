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
type Account struct {
	Type               string
	RequiresOpenAIAuth bool
}

func (a Account) Authenticated() bool {
	return !a.RequiresOpenAIAuth || a.Type != ""
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
type AgentMessageDelta struct {
	ItemID string
	Delta  string
}
type TurnCompletion struct {
	Status       string
	ErrorMessage string
	FinalMessage Item
}
type MCPServer struct {
	Name  string
	Tools map[string]json.RawMessage
}
type DynamicTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}
type DynamicToolNamespace struct {
	Type        string        `json:"type"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Tools       []DynamicTool `json:"tools"`
}
type MCPToolResult struct {
	Content []json.RawMessage `json:"content"`
	IsError bool              `json:"isError,omitempty"`
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
	var r struct{}
	if err := a.call(ctx, "initialize", map[string]any{"clientInfo": map[string]string{"name": in.ClientName, "version": in.ClientVersion}, "capabilities": map[string]any{"experimentalApi": in.Experimental}}, &r, true); err != nil {
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
func (a *API) Account(ctx context.Context) (Account, error) {
	if err := a.require(); err != nil {
		return Account{}, err
	}
	b, err := a.c.Call(ctx, "account/read", map[string]bool{"refreshToken": false}, true)
	if err != nil {
		return Account{}, err
	}
	return DecodeAccount(b)
}
func (a *API) StartThread(ctx context.Context, cwd, model, developerInstructions string, dynamicTools []DynamicToolNamespace) (Thread, error) {
	if err := a.require(); err != nil {
		return Thread{}, err
	}
	var r struct {
		Thread Thread `json:"thread"`
	}
	params := map[string]any{"cwd": cwd, "model": model}
	if developerInstructions != "" {
		params["developerInstructions"] = developerInstructions
	}
	if len(dynamicTools) > 0 {
		params["dynamicTools"] = dynamicTools
	}
	err := a.call(ctx, "thread/start", params, &r, false)
	return r.Thread, err
}
func (a *API) ResumeThread(ctx context.Context, id, model, developerInstructions string, dynamicTools []DynamicToolNamespace) (Thread, error) {
	if err := a.require(); err != nil {
		return Thread{}, err
	}
	var r struct {
		Thread Thread `json:"thread"`
	}
	params := map[string]any{"threadId": id, "model": model}
	if developerInstructions != "" {
		params["developerInstructions"] = developerInstructions
	}
	if len(dynamicTools) > 0 {
		params["dynamicTools"] = dynamicTools
	}
	err := a.call(ctx, "thread/resume", params, &r, false)
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
func (a *API) StartTurn(ctx context.Context, threadID, text, clientMessageID, model string) (Turn, error) {
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
	if model != "" {
		p["model"] = model
	}
	err := a.call(ctx, "turn/start", p, &r, false)
	return r.Turn, err
}
func (a *API) ListMCPServers(ctx context.Context, threadID string) ([]MCPServer, error) {
	if err := a.require(); err != nil {
		return nil, err
	}
	var response struct {
		Data []struct {
			Name  string                     `json:"name"`
			Tools map[string]json.RawMessage `json:"tools"`
		} `json:"data"`
	}
	err := a.call(ctx, "mcpServerStatus/list", map[string]any{
		"threadId": threadID,
		"detail":   "toolsAndAuthOnly",
	}, &response, true)
	if err != nil {
		return nil, err
	}
	servers := make([]MCPServer, 0, len(response.Data))
	for _, server := range response.Data {
		servers = append(servers, MCPServer{Name: server.Name, Tools: server.Tools})
	}
	return servers, nil
}
func (a *API) CallMCPTool(ctx context.Context, threadID, server, tool string, arguments json.RawMessage) (MCPToolResult, error) {
	if err := a.require(); err != nil {
		return MCPToolResult{}, err
	}
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	var result MCPToolResult
	err := a.call(ctx, "mcpServer/tool/call", map[string]any{
		"threadId":  threadID,
		"server":    server,
		"tool":      tool,
		"arguments": arguments,
	}, &result, false)
	return result, err
}
func (a *API) Interrupt(ctx context.Context, threadID, turnID string) error {
	if err := a.require(); err != nil {
		return err
	}
	return a.call(ctx, "turn/interrupt", map[string]string{"threadId": threadID, "turnId": turnID}, nil, false)
}
func (a *API) Unsubscribe(ctx context.Context, threadID string) error {
	if err := a.require(); err != nil {
		return err
	}
	return a.call(ctx, "thread/unsubscribe", map[string]string{"threadId": threadID}, nil, false)
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

func DecodeAccount(result json.RawMessage) (Account, error) {
	var wire struct {
		Account *struct {
			Type string `json:"type"`
		} `json:"account"`
		RequiresOpenAIAuth bool `json:"requiresOpenaiAuth"`
	}
	if err := json.Unmarshal(result, &wire); err != nil {
		return Account{}, err
	}
	account := Account{RequiresOpenAIAuth: wire.RequiresOpenAIAuth}
	if wire.Account != nil {
		account.Type = wire.Account.Type
	}
	return account, nil
}

func DecodeAgentMessageDelta(params json.RawMessage) (AgentMessageDelta, error) {
	var wire struct {
		ItemID string `json:"itemId"`
		Delta  string `json:"delta"`
	}
	if err := json.Unmarshal(params, &wire); err != nil {
		return AgentMessageDelta{}, err
	}
	if wire.ItemID == "" {
		return AgentMessageDelta{}, errors.New("agent message delta is missing item ID")
	}
	return AgentMessageDelta{ItemID: wire.ItemID, Delta: wire.Delta}, nil
}

func DecodeTurnCompletion(params json.RawMessage) (TurnCompletion, error) {
	var wire struct {
		Turn struct {
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
			Items []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"items"`
		} `json:"turn"`
	}
	if err := json.Unmarshal(params, &wire); err != nil {
		return TurnCompletion{}, err
	}
	completion := TurnCompletion{Status: wire.Turn.Status}
	if wire.Turn.Error != nil {
		completion.ErrorMessage = wire.Turn.Error.Message
	}
	for _, item := range wire.Turn.Items {
		if item.Type == "agentMessage" {
			completion.FinalMessage = Item{ID: item.ID, Type: item.Type, Text: item.Text}
		}
	}
	return completion, nil
}

func DecodeErrorMessage(params json.RawMessage) (string, error) {
	var wire struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(params, &wire); err != nil {
		return "", err
	}
	if wire.Error.Message == "" {
		return "", errors.New("error event is missing a message")
	}
	return wire.Error.Message, nil
}
