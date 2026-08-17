package appserver

import (
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
