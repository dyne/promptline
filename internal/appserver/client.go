package appserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

type call struct {
	result  json.RawMessage
	err     error
	done    chan struct{}
	bytes   int
	release sync.Once
}

type queuedEvent struct {
	event Event
	bytes int
}

type queuedRequest struct {
	request ServerRequest
	bytes   int
}

// Client has exactly one stdout reader and serializes all stdin writes.
type Client struct {
	in        io.WriteCloser
	closeIn   func() error
	limits    Limits
	writeMu   sync.Mutex
	mu        sync.Mutex
	nextID    uint64
	pending   map[uint64]*call
	events    chan Event
	requests  chan ServerRequest
	eventsQ   chan queuedEvent
	requestsQ chan queuedRequest
	inbound   int
	eventN    int
	requestN  int
	done      chan struct{}
	closeOnce sync.Once
	fatal     error
}

func New(stdin io.WriteCloser, stdout io.Reader, cfg Config) *Client {
	l := cfg.Limits.normalized()
	c := &Client{in: stdin, limits: l, pending: make(map[uint64]*call), events: make(chan Event), requests: make(chan ServerRequest), eventsQ: make(chan queuedEvent, l.MaxEvents), requestsQ: make(chan queuedRequest, l.MaxServerRequests), done: make(chan struct{})}
	go c.deliverEvents()
	go c.deliverRequests()
	go c.read(stdout)
	return c
}
func (c *Client) Events() <-chan Event           { return c.events }
func (c *Client) Requests() <-chan ServerRequest { return c.requests }
func (c *Client) Done() <-chan struct{}          { return c.done }
func (c *Client) Err() error                     { c.mu.Lock(); defer c.mu.Unlock(); return c.fatal }

func (c *Client) Call(ctx context.Context, method string, params any, idempotentRead bool) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", method, err)
	}
	c.mu.Lock()
	if c.fatal != nil {
		err = c.fatal
		c.mu.Unlock()
		return nil, err
	}
	if len(c.pending) >= c.limits.MaxPending {
		c.mu.Unlock()
		return nil, fmt.Errorf("%w: pending call limit", ErrOverloaded)
	}
	c.nextID++
	id := c.nextID
	p := &call{done: make(chan struct{})}
	c.pending[id] = p
	c.mu.Unlock()
	if err = c.write(envelope{ID: &id, Method: method, Params: body}); err != nil {
		c.remove(id, err)
		return nil, err
	}
	select {
	case <-ctx.Done():
		c.remove(id, ctx.Err())
		return nil, ctx.Err()
	case <-c.done:
		c.remove(id, ErrClosed)
		return nil, c.Err()
	case <-p.done:
		c.releaseCall(p)
		return p.result, p.err
	}
}
func (c *Client) Notify(method string, params any) error {
	b, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(envelope{Method: method, Params: b})
}
func (c *Client) Reply(ctx context.Context, id uint64, result any, rpcErr *RPCError) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return c.write(envelope{ID: &id, Result: b, Error: rpcErr})
}
func (c *Client) write(e envelope) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	select {
	case <-c.done:
		return ErrClosed
	default:
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if len(b) > c.limits.MaxFrameBytes {
		return fmt.Errorf("%w: frame too large", ErrProtocol)
	}
	_, err = c.in.Write(b)
	if err != nil {
		c.fail(fmt.Errorf("write app-server stdin: %w", err))
	}
	return err
}
func (c *Client) read(r io.Reader) {
	s := bufio.NewScanner(r)
	initial := 4096
	if c.limits.MaxFrameBytes < initial {
		initial = c.limits.MaxFrameBytes
	}
	s.Buffer(make([]byte, initial), c.limits.MaxFrameBytes)
	for s.Scan() {
		var e envelope
		if err := json.Unmarshal(s.Bytes(), &e); err != nil {
			c.fail(fmt.Errorf("%w: invalid JSON: %v", ErrProtocol, err))
			return
		}
		c.dispatch(e)
	}
	if err := s.Err(); err != nil {
		c.fail(fmt.Errorf("%w: read stdout: %v", ErrProtocol, err))
	} else {
		c.fail(io.EOF)
	}
}
func (c *Client) dispatch(e envelope) {
	if e.ID != nil && e.Method != "" {
		bytes := inboundSize(e.Method, e.Params)
		if !c.reserveQueue(bytes, true) {
			c.fail(fmt.Errorf("%w: server request queue full or inbound byte budget", ErrOverloaded))
			return
		}
		select {
		case c.requestsQ <- queuedRequest{request: ServerRequest{ID: *e.ID, Method: e.Method, Params: e.Params}, bytes: bytes}:
		default:
			c.releaseQueue(bytes, true)
			c.fail(fmt.Errorf("%w: server request queue full", ErrOverloaded))
		}
		return
	}
	if e.ID != nil {
		bytes := inboundSize("", e.Result)
		if e.Error != nil {
			bytes += inboundSize(e.Error.Message, e.Error.Data)
		}
		if !c.reserveInbound(bytes) {
			c.fail(fmt.Errorf("%w: inbound byte budget", ErrOverloaded))
			return
		}
		c.mu.Lock()
		p := c.pending[*e.ID]
		delete(c.pending, *e.ID)
		c.mu.Unlock()
		if p == nil {
			c.releaseInbound(bytes)
			return
		}
		p.result = e.Result
		p.bytes = bytes
		if e.Error != nil {
			p.err = e.Error
		}
		close(p.done)
		return
	}
	if e.Method != "" {
		bytes := inboundSize(e.Method, e.Params)
		if !c.reserveQueue(bytes, false) {
			c.fail(fmt.Errorf("%w: event queue full or inbound byte budget", ErrOverloaded))
			return
		}
		select {
		case c.eventsQ <- queuedEvent{event: Event{Method: e.Method, Params: e.Params}, bytes: bytes}:
		default:
			c.releaseQueue(bytes, false)
			c.fail(fmt.Errorf("%w: event queue full", ErrOverloaded))
		}
		return
	}
	c.fail(fmt.Errorf("%w: malformed envelope", ErrProtocol))
}
func (c *Client) remove(id uint64, err error) {
	c.mu.Lock()
	p := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if p != nil {
		c.releaseCall(p)
		p.err = err
		close(p.done)
	}
}

func (c *Client) releaseCall(p *call) { p.release.Do(func() { c.releaseInbound(p.bytes) }) }

func inboundSize(method string, payload json.RawMessage) int { return len(method) + len(payload) }

func (c *Client) reserveInbound(bytes int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fatal != nil || bytes > c.limits.MaxQueuedBytes-c.inbound {
		return false
	}
	c.inbound += bytes
	return true
}

func (c *Client) releaseInbound(bytes int) {
	if bytes <= 0 {
		return
	}
	c.mu.Lock()
	c.inbound -= bytes
	c.mu.Unlock()
}

func (c *Client) reserveQueue(bytes int, request bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fatal != nil || bytes > c.limits.MaxQueuedBytes-c.inbound {
		return false
	}
	if request {
		if c.requestN >= c.limits.MaxServerRequests {
			return false
		}
		c.requestN++
	} else {
		if c.eventN >= c.limits.MaxEvents {
			return false
		}
		c.eventN++
	}
	c.inbound += bytes
	return true
}

func (c *Client) releaseQueue(bytes int, request bool) {
	c.mu.Lock()
	c.inbound -= bytes
	if request {
		c.requestN--
	} else {
		c.eventN--
	}
	c.mu.Unlock()
}

func (c *Client) deliverEvents() {
	for {
		select {
		case <-c.done:
			return
		case queued := <-c.eventsQ:
			c.releaseInbound(queued.bytes)
			select {
			case <-c.done:
				return
			case c.events <- queued.event:
				c.releaseQueue(0, false)
			}
		}
	}
}

func (c *Client) deliverRequests() {
	for {
		select {
		case <-c.done:
			return
		case queued := <-c.requestsQ:
			c.releaseInbound(queued.bytes)
			select {
			case <-c.done:
				return
			case c.requests <- queued.request:
				c.releaseQueue(0, true)
			}
		}
	}
}
func (c *Client) fail(err error) {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.fatal = err
		ps := c.pending
		c.pending = make(map[uint64]*call)
		c.mu.Unlock()
		for _, p := range ps {
			c.releaseCall(p)
			p.err = err
			close(p.done)
		}
		close(c.done)
		if c.in != nil {
			_ = c.in.Close()
		}
	})
}
func (c *Client) Close() error { c.fail(ErrClosed); return nil }

var _ = errors.Is
