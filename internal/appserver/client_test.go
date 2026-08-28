package appserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"
)

type pipeWriteCloser struct{ io.Writer }

func (pipeWriteCloser) Close() error { return nil }

func TestClient_CorrelationNotificationsAndServerRequest(t *testing.T) {
	client, server := io.Pipe()
	inR, inW := io.Pipe()
	c := New(pipeWriteCloser{inW}, client, Config{})
	defer c.Close()
	result := make(chan error, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 128)
		n, _ := inR.Read(buf)
		b.Write(buf[:n])
		var req envelope
		if err := json.Unmarshal([]byte(b.String()), &req); err != nil {
			result <- err
			return
		}
		_, _ = server.Write([]byte(`{"method":"item/agentMessage/delta","params":{"delta":"hello","extra":true}}` + "\n"))
		_, _ = server.Write([]byte(`{"id":99,"method":"item/commandExecution/requestApproval","params":{"threadId":"t"}}` + "\n"))
		_, _ = server.Write([]byte(`{"id":` + jsonNumber(*req.ID) + `,"result":{"ok":true,"unknown":"accepted"}}` + "\n"))
		result <- nil
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := c.Call(ctx, "account/read", map[string]bool{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ok":true,"unknown":"accepted"}` {
		t.Fatalf("result=%s", got)
	}
	select {
	case e := <-c.Events():
		if e.Method != "item/agentMessage/delta" {
			t.Fatal(e.Method)
		}
	case <-ctx.Done():
		t.Fatal("event timeout")
	}
	select {
	case r := <-c.Requests():
		if r.ID != 99 {
			t.Fatal(r.ID)
		}
	case <-ctx.Done():
		t.Fatal("request timeout")
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
func jsonNumber(v uint64) string {
	return string(json.RawMessage([]byte("" + strconv.FormatUint(v, 10))))
}

func TestClient_OverloadAndMalformed(t *testing.T) {
	out, _ := io.Pipe()
	_, in := io.Pipe()
	c := New(pipeWriteCloser{in}, out, Config{Limits: Limits{MaxPending: 1}})
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go c.Call(ctx, "account/read", nil, true)
	time.Sleep(time.Millisecond)
	if _, err := c.Call(ctx, "account/read", nil, true); err == nil {
		t.Fatal("want pending overload")
	}
	badR, badW := io.Pipe()
	_, badIn := io.Pipe()
	bad := New(pipeWriteCloser{badIn}, badR, Config{})
	_, _ = badW.Write([]byte("not-json\n"))
	select {
	case <-bad.Done():
		if !errors.Is(bad.Err(), ErrProtocol) {
			t.Fatalf("%v", bad.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("not closed")
	}
}

func TestClient_TerminationAndQueueBounds(t *testing.T) {
	tests := []struct {
		name   string
		limits Limits
		frames []string
		want   error
	}{
		{name: "event queue", limits: Limits{MaxEvents: 1}, frames: []string{`{"method":"one"}`, `{"method":"two"}`}, want: ErrOverloaded},
		{name: "request queue", limits: Limits{MaxServerRequests: 1}, frames: []string{`{"id":1,"method":"one"}`, `{"id":2,"method":"two"}`}, want: ErrOverloaded},
		{name: "inbound byte budget", limits: Limits{MaxQueuedBytes: 5}, frames: []string{`{"method":"one","params":"1234567890"}`}, want: ErrOverloaded},
		{name: "malformed envelope", frames: []string{`{"params":{}}`}, want: ErrProtocol},
		{name: "oversized frame", limits: Limits{MaxFrameBytes: 16}, frames: []string{`{"method":"this-is-too-large"}`}, want: ErrProtocol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, server := io.Pipe()
			c := New(pipeWriteCloser{io.Discard}, out, Config{Limits: tt.limits})
			defer c.Close()
			written := make(chan error, 1)
			go func() {
				for _, frame := range tt.frames {
					if _, err := io.WriteString(server, frame+"\n"); err != nil {
						written <- err
						return
					}
				}
				written <- server.Close()
			}()
			select {
			case <-c.Done():
				if !errors.Is(c.Err(), tt.want) {
					t.Fatalf("Err() = %v, want %v", c.Err(), tt.want)
				}
			case <-time.After(time.Second):
				t.Fatal("client did not terminate")
			}
			_ = server.Close()
			<-written
		})
	}
}

func TestClient_CallCancellationEOFAndWriteFailure(t *testing.T) {
	t.Run("canceled before write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c := New(pipeWriteCloser{io.Discard}, strings.NewReader(""), Config{})
		defer c.Close()
		if _, err := c.Call(ctx, "read", nil, true); !errors.Is(err, context.Canceled) {
			t.Fatalf("Call() error = %v", err)
		}
	})
	t.Run("canceled after write removes pending", func(t *testing.T) {
		inR, inW := io.Pipe()
		outR, _ := io.Pipe()
		c := New(pipeWriteCloser{inW}, outR, Config{})
		defer c.Close()
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() { _, err := c.Call(ctx, "read", nil, true); result <- err }()
		buf := make([]byte, 128)
		if _, err := inR.Read(buf); err != nil {
			t.Fatal(err)
		}
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("Call() error = %v", err)
		}
		c.mu.Lock()
		pending := len(c.pending)
		c.mu.Unlock()
		if pending != 0 {
			t.Fatalf("pending = %d", pending)
		}
	})
	t.Run("EOF wakes pending calls", func(t *testing.T) {
		inR, inW := io.Pipe()
		outR, outW := io.Pipe()
		c := New(pipeWriteCloser{inW}, outR, Config{})
		defer c.Close()
		result := make(chan error, 1)
		go func() { _, err := c.Call(context.Background(), "read", nil, true); result <- err }()
		buf := make([]byte, 128)
		if _, err := inR.Read(buf); err != nil {
			t.Fatal(err)
		}
		if err := outW.Close(); err != nil {
			t.Fatal(err)
		}
		if err := <-result; !errors.Is(err, io.EOF) {
			t.Fatalf("Call() error = %v", err)
		}
	})
	t.Run("write failure terminates client", func(t *testing.T) {
		c := New(pipeWriteCloser{errWriter{}}, strings.NewReader(""), Config{})
		if _, err := c.Call(context.Background(), "read", nil, true); err == nil {
			t.Fatal("Call() succeeded")
		}
		select {
		case <-c.Done():
		case <-time.After(time.Second):
			t.Fatal("client did not close")
		}
	})
}

func TestClient_OutOfOrderDuplicateAndIdempotentClose(t *testing.T) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := New(pipeWriteCloser{inW}, outR, Config{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	results := make(chan json.RawMessage, 2)
	for range 2 {
		go func() {
			got, err := c.Call(ctx, "read", nil, true)
			if err != nil {
				t.Errorf("Call: %v", err)
				return
			}
			results <- got
		}()
	}
	s := bufio.NewScanner(inR)
	ids := make([]uint64, 0, 2)
	for len(ids) < 2 {
		if !s.Scan() {
			t.Fatal("missing request")
		}
		var e envelope
		if err := json.Unmarshal(s.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, *e.ID)
	}
	for _, frame := range []string{`{"id":999,"result":{"ignored":true}}`, `{"id":` + strconv.FormatUint(ids[1], 10) + `,"result":{"order":2}}`, `{"id":` + strconv.FormatUint(ids[0], 10) + `,"result":{"order":1}}`, `{"id":` + strconv.FormatUint(ids[0], 10) + `,"result":{"duplicate":true}}`} {
		if _, err := io.WriteString(outW, frame+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	got := []string{string(<-results), string(<-results)}
	if !(got[0] == `{"order":1}` || got[1] == `{"order":1}`) {
		t.Fatalf("results = %v", got)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

func FuzzClientDispatch(f *testing.F) {
	for _, seed := range []string{`{"method":"event","params":{}}`, `{"id":1,"result":null}`, `{"id":1,"method":"request"}`, `{}`, `not-json`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, frame string) {
		if len(frame) > 4096 {
			t.Skip()
		}
		outR, outW := io.Pipe()
		c := New(pipeWriteCloser{io.Discard}, outR, Config{Limits: Limits{MaxFrameBytes: 4096, MaxEvents: 2, MaxServerRequests: 2}})
		_, _ = io.WriteString(outW, frame+"\n")
		_ = outW.Close()
		select {
		case <-c.Done():
		case <-time.After(time.Second):
			t.Fatal("dispatch deadlocked")
		}
	})
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("stdin failed") }
