package runtime

import (
	"sync"

	"promptline/internal/appserver"
)

// runtimeState is the synchronized state machine for one interactive session.
// Keeping it separate from protocol and console adapters makes the ownership of
// thread selection, active-turn admission, and event reduction explicit.
type runtimeState struct {
	mu sync.Mutex

	threadID string
	turnID   string
	starting bool
	closing  bool

	streamedAgentItems map[string]struct{}
	turnHasOutput      bool
	streamOpen         bool
	turnErrorRendered  bool
	pendingItems       map[string]appserver.Item
}

func newRuntimeState() runtimeState {
	return runtimeState{
		streamedAgentItems: make(map[string]struct{}),
		pendingItems:       make(map[string]appserver.Item),
	}
}

func (s *runtimeState) thread() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.threadID
}

func (s *runtimeState) setThread(id string) {
	s.mu.Lock()
	s.threadID = id
	s.mu.Unlock()
}

func (s *runtimeState) beginTurn() (threadID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return "", ErrShuttingDown
	}
	if s.turnID != "" || s.starting {
		return "", ErrActiveTurn
	}
	if s.threadID == "" {
		return "", errNoPrimaryThread
	}
	s.starting = true
	return s.threadID, nil
}

func (s *runtimeState) rejectTurn() { s.mu.Lock(); s.starting = false; s.mu.Unlock() }

func (s *runtimeState) acceptTurn(id string) {
	s.mu.Lock()
	s.turnID = id
	s.starting = false
	s.streamedAgentItems = make(map[string]struct{})
	s.pendingItems = make(map[string]appserver.Item)
	s.turnHasOutput = false
	s.streamOpen = false
	s.turnErrorRendered = false
	s.mu.Unlock()
}

func (s *runtimeState) rememberPending(item appserver.Item) {
	s.mu.Lock()
	s.pendingItems[item.ID] = item
	s.mu.Unlock()
}

func (s *runtimeState) pendingItem(id string) appserver.Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingItems[id]
}

func (s *runtimeState) activeTurn() (threadID, turnID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.threadID, s.turnID
}

func (s *runtimeState) hasActiveTurn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.turnID != ""
}

func (s *runtimeState) completeTurn(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.turnID != id || id == "" {
		return false
	}
	s.turnID = ""
	return true
}

func (s *runtimeState) beginShutdown() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closing = true
	return s.threadID
}

func (s *runtimeState) markDelta(itemID string) {
	s.mu.Lock()
	s.streamedAgentItems[itemID] = struct{}{}
	s.turnHasOutput = true
	s.streamOpen = true
	s.mu.Unlock()
}

func (s *runtimeState) reduceItem(itemID, text string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, streamed := s.streamedAgentItems[itemID]
	if !streamed && text != "" {
		s.turnHasOutput = true
	}
	return streamed || text == ""
}

func (s *runtimeState) beginCompletion() (turnID string, streamOpen, hasOutput, errorRendered bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	turnID, streamOpen, hasOutput, errorRendered = s.turnID, s.streamOpen, s.turnHasOutput, s.turnErrorRendered
	s.streamOpen = false
	return turnID, streamOpen, hasOutput, errorRendered
}

func (s *runtimeState) beginError() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	streamOpen := s.streamOpen
	s.streamOpen = false
	s.turnErrorRendered = true
	return streamOpen
}
