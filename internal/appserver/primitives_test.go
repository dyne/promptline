package appserver

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestAPI_Lifecycle(t *testing.T) {
	fromClient, toServer := io.Pipe()
	fromServer, toClient := io.Pipe()
	c := New(pipeWriteCloser{toServer}, fromServer, Config{})
	defer c.Close()
	go func() {
		s := bufio.NewScanner(fromClient)
		for s.Scan() {
			var e envelope
			if json.Unmarshal(s.Bytes(), &e) != nil || e.ID == nil {
				continue
			}
			var result string
			switch e.Method {
			case "initialize":
				result = `{}`
			case "thread/start", "thread/resume", "thread/read":
				result = `{"thread":{"id":"thr_1","status":"idle"}}`
			case "turn/start":
				result = `{"turn":{"id":"turn_1","status":"inProgress"}}`
			default:
				result = `{}`
			}
			_, _ = toClient.Write([]byte(`{"id":` + jsonNumber(*e.ID) + `,"result":` + result + "}\n"))
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	a := NewAPI(c)
	if err := a.Initialize(ctx, Initialize{ClientName: "promptline", ClientVersion: "v2"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Initialize(ctx, Initialize{}); err == nil {
		t.Fatal("double initialize accepted")
	}
	thread, err := a.StartThread(ctx, "/tmp", "")
	if err != nil || thread.ID != "thr_1" {
		t.Fatalf("thread=%+v err=%v", thread, err)
	}
	turn, err := a.StartTurn(ctx, thread.ID, "hello", "client_1")
	if err != nil || turn.ID != "turn_1" {
		t.Fatalf("turn=%+v err=%v", turn, err)
	}
	if err := a.Interrupt(ctx, thread.ID, turn.ID); err != nil {
		t.Fatal(err)
	}
}

func TestAPI_RejectsExperimentalAndUninitialized(t *testing.T) {
	c := New(pipeWriteCloser{io.Discard}, &emptyReader{}, Config{})
	defer c.Close()
	a := NewAPI(c)
	ctx := context.Background()
	if _, err := a.ReadThread(ctx, "thr"); err != ErrNotInitialized {
		t.Fatalf("got %v", err)
	}
	if err := a.Initialize(ctx, Initialize{Experimental: true}); err == nil {
		t.Fatal("experimental accepted")
	}
}

func TestDecodeItemAndReplyOnce(t *testing.T) {
	item, err := DecodeItem(json.RawMessage(`{"threadId":"thr","turnId":"turn","item":{"id":"item","type":"agentMessage","text":"hi","newField":true}}`))
	if err != nil || item.ID != "item" || item.ThreadID != "thr" {
		t.Fatalf("item=%+v err=%v", item, err)
	}
	inR, inW := io.Pipe()
	outR, _ := io.Pipe()
	c := New(pipeWriteCloser{inW}, outR, Config{})
	defer c.Close()
	a := NewAPI(c)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	read := make(chan string, 1)
	go func() {
		buf := make([]byte, 256)
		n, _ := inR.Read(buf)
		read <- string(buf[:n])
	}()
	if err := a.ReplyRequest(ctx, 7, map[string]string{"decision": "decline"}); err != nil {
		t.Fatal(err)
	}
	if err := a.ReplyRequest(ctx, 7, map[string]string{}); err == nil {
		t.Fatal("duplicate reply accepted")
	}
	if got := <-read; !strings.Contains(got, `"id":7`) {
		t.Fatalf("reply=%s", got)
	}
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) { return 0, io.EOF }
