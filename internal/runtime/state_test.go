package runtime

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"promptline/internal/appserver"
)

func TestRuntimeStateTurnLifecycle(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*runtimeState)
		wantErr    error
		wantThread string
	}{
		{
			name:    "no selected thread rejects a turn",
			wantErr: errNoPrimaryThread,
		},
		{
			name: "selected thread admits a turn",
			setup: func(state *runtimeState) {
				state.setThread("thread-1")
			},
			wantThread: "thread-1",
		},
		{
			name: "active turn rejects a second turn",
			setup: func(state *runtimeState) {
				state.setThread("thread-1")
				state.acceptTurn("turn-1")
			},
			wantErr: ErrActiveTurn,
		},
		{
			name: "shutdown rejects a turn",
			setup: func(state *runtimeState) {
				state.setThread("thread-1")
				state.beginShutdown()
			},
			wantErr: ErrShuttingDown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newRuntimeState()
			if test.setup != nil {
				test.setup(&state)
			}
			threadID, err := state.beginTurn()
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("beginTurn() error = %v, want %v", err, test.wantErr)
			}
			if threadID != test.wantThread {
				t.Fatalf("beginTurn() thread = %q, want %q", threadID, test.wantThread)
			}
		})
	}
}

func TestRuntimeStateSerializesConcurrentTurnAdmission(t *testing.T) {
	state := newRuntimeState()
	state.setThread("thread-1")

	const attempts = 100
	var admitted atomic.Int32
	var group sync.WaitGroup
	for range attempts {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := state.beginTurn()
			if err != nil {
				return
			}
			state.acceptTurn("turn-1")
			admitted.Add(1)
			state.completeTurn("turn-1")
		}()
	}
	group.Wait()
	if state.hasActiveTurn() {
		t.Fatal("concurrent admission left a turn active")
	}
	if admitted.Load() == 0 {
		t.Fatal("no concurrent caller was admitted")
	}
}

func TestRuntimeStateEventReduction(t *testing.T) {
	state := newRuntimeState()
	state.setThread("thread-1")
	state.acceptTurn("turn-1")

	state.markDelta("item-1")
	if suppress := state.reduceItem("item-1", "streamed response"); !suppress {
		t.Fatal("streamed item completion must be suppressed")
	}
	if suppress := state.reduceItem("item-2", "fallback response"); suppress {
		t.Fatal("unstreamed item completion must be emitted")
	}
	turnID, streamOpen, hasOutput, errorRendered := state.beginCompletion()
	if turnID != "turn-1" || !streamOpen || !hasOutput || errorRendered {
		t.Fatalf("completion state = (%q, %v, %v, %v)", turnID, streamOpen, hasOutput, errorRendered)
	}
	if !state.completeTurn(turnID) || state.hasActiveTurn() {
		t.Fatal("matching completion must end the active turn")
	}
}

func TestRuntimeStatePendingItems(t *testing.T) {
	state := newRuntimeState()
	state.rememberPending(appserver.Item{
		ID:  "item-1",
		Raw: json.RawMessage(`{"id":"item-1"}`),
	})

	if got := state.pendingItem("item-1"); got.ID != "item-1" || string(got.Raw) != `{"id":"item-1"}` {
		t.Fatalf("pendingItem() = %#v", got)
	}
	if got := state.pendingItem("missing"); got.ID != "" || got.Raw != nil {
		t.Fatalf("missing pendingItem() = %#v", got)
	}

	state.acceptTurn("turn-2")
	if got := state.pendingItem("item-1"); got.ID != "" || got.Raw != nil {
		t.Fatalf("pending item survived turn reset: %#v", got)
	}
}
