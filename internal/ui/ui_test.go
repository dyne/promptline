package ui

import (
	"context"
	"testing"
	"time"

	"promptline/internal/chat"
	"promptline/internal/config"
	"promptline/internal/theme"
)

func TestHistoryNavigation(t *testing.T) {
	h := NewHistory([]string{"one", "two", "three"})

	if entry, ok := h.Prev(); !ok || entry != "three" {
		t.Fatalf("expected prev to return last entry 'three', got '%s' (ok=%v)", entry, ok)
	}
	if entry, ok := h.Prev(); !ok || entry != "two" {
		t.Fatalf("expected prev to move back to 'two', got '%s' (ok=%v)", entry, ok)
	}
	if entry, ok := h.Prev(); !ok || entry != "one" {
		t.Fatalf("expected prev to move back to 'one', got '%s' (ok=%v)", entry, ok)
	}
	if entry, ok := h.Prev(); !ok || entry != "one" {
		t.Fatalf("expected prev at start to stay on 'one', got '%s' (ok=%v)", entry, ok)
	}

	if entry, ok := h.Next(); !ok || entry != "two" {
		t.Fatalf("expected next to advance to 'two', got '%s' (ok=%v)", entry, ok)
	}
	if entry, ok := h.Next(); !ok || entry != "three" {
		t.Fatalf("expected next to advance to 'three', got '%s' (ok=%v)", entry, ok)
	}
	if entry, ok := h.Next(); !ok || entry != "" {
		t.Fatalf("expected next to clear at end, got '%s' (ok=%v)", entry, ok)
	}
	if _, ok := h.Next(); ok {
		t.Fatalf("expected no movement after clear")
	}
}

func TestComputeElasticHeight(t *testing.T) {
	tests := []struct {
		lines    int
		expected int
	}{
		{1, 5},
		{5, 5},
		{6, 10},
		{10, 10},
		{11, 15},
		{20, 15},
	}

	for _, tt := range tests {
		if got := computeElasticHeight(tt.lines, 5, 15); got != tt.expected {
			t.Fatalf("computeElasticHeight(%d) = %d, expected %d", tt.lines, got, tt.expected)
		}
	}
}

func TestBackgroundWorkersStopOnCancel(t *testing.T) {
	session := chat.NewSession(config.DefaultConfig())
	defer session.Close()
	ui := New(session, theme.DefaultTheme())

	ctx, cancel := context.WithCancel(context.Background())
	ui.startBackgroundWorkers(ctx)
	cancel()

	done := make(chan struct{})
	go func() {
		ui.bgWG.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("background workers did not stop after cancel")
	}
}
