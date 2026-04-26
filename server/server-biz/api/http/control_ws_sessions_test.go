package httpapi

import (
	"testing"

	"golang.org/x/net/websocket"
)

func TestControlWSHubUnregisterKeepsReplacementSession(t *testing.T) {
	hub := newControlWSHub()
	oldConn := &websocket.Conn{}
	newConn := &websocket.Conn{}

	hub.register(&controlWSSession{conn: oldConn, nodeID: "node-1"})
	hub.register(&controlWSSession{conn: newConn, nodeID: "node-1"})

	hub.unregister("node-1", oldConn)
	if got := hub.session("node-1"); got == nil || got.conn != newConn {
		t.Fatalf("expected replacement session to remain, got %+v", got)
	}

	hub.unregister("node-1", newConn)
	if got := hub.session("node-1"); got != nil {
		t.Fatalf("expected replacement session to be removed, got %+v", got)
	}
}
