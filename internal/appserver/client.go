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
	result json.RawMessage
	err    error
	done   chan struct{}
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
	done      chan struct{}
	closeOnce sync.Once
	fatal     error
}

func New(stdin io.WriteCloser, stdout io.Reader, cfg Config) *Client {
	l := cfg.Limits.normalized()
	c := &Client{in: stdin, limits: l, pending: make(map[uint64]*call), events: make(chan Event, l.MaxEvents), requests: make(chan ServerRequest, l.MaxServerRequests), done: make(chan struct{})}
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
	s.Buffer(make([]byte, 4096), c.limits.MaxFrameBytes)
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
		select {
		case c.requests <- ServerRequest{ID: *e.ID, Method: e.Method, Params: e.Params}:
		default:
			c.fail(fmt.Errorf("%w: server request queue full", ErrOverloaded))
		}
		return
	}
	if e.ID != nil {
		c.mu.Lock()
		p := c.pending[*e.ID]
		delete(c.pending, *e.ID)
		c.mu.Unlock()
		if p == nil {
			return
		}
		p.result = e.Result
		if e.Error != nil {
			p.err = e.Error
		}
		close(p.done)
		return
	}
	if e.Method != "" {
		select {
		case c.events <- Event{Method: e.Method, Params: e.Params}:
		default:
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
		p.err = err
		close(p.done)
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
