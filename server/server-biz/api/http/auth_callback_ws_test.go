package httpapi

import (
	"testing"

	"golang.org/x/net/websocket"
)

func TestAuthCallbackWSHubRegisterUnregister(t *testing.T) {
	hub := newAuthCallbackWSHub()
	conn := &websocket.Conn{}

	hub.register("cb-1", conn)
	if got := hub.sessionCount("cb-1"); got != 1 {
		t.Fatalf("expected one session, got %d", got)
	}
	hub.unregister("cb-1", conn)
	if got := hub.sessionCount("cb-1"); got != 0 {
		t.Fatalf("expected no sessions, got %d", got)
	}
}

func (h *authCallbackWSHub) sessionCount(callbackID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions[callbackID])
}
