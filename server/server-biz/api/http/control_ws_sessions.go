package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	controlws "github.com/slan/server/server-biz/internal/ws"
	"golang.org/x/net/websocket"
)

type wsSession struct {
	userID    string
	deviceID  string
	nodeID    string
	networkID string
}

type controlWSSession struct {
	conn      *websocket.Conn
	writeMu   sync.Mutex
	userID    string
	deviceID  string
	nodeID    string
	networkID string
}

type controlWSHub struct {
	mu       sync.RWMutex
	sessions map[string]*controlWSSession
}

func newControlWSHub() *controlWSHub {
	return &controlWSHub{
		sessions: make(map[string]*controlWSSession),
	}
}

func (h *controlWSHub) register(session *controlWSSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[session.nodeID] = session
	controlWSActiveSessions.Set(int64(len(h.sessions)))
	metricAdd("session_register_total", 1)
}

func (h *controlWSHub) unregister(nodeID string, conn ...*websocket.Conn) {
	if nodeID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(conn) > 0 {
		current := h.sessions[nodeID]
		if current != nil && current.conn != conn[0] {
			return
		}
	}
	delete(h.sessions, nodeID)
	controlWSActiveSessions.Set(int64(len(h.sessions)))
	metricAdd("session_unregister_total", 1)
}

func (h *controlWSHub) session(nodeID string) *controlWSSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[nodeID]
}

func (h *controlWSHub) peersInNetwork(networkID, excludeNodeID string) []*controlWSSession {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]*controlWSSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		if session.networkID != networkID || session.nodeID == excludeNodeID {
			continue
		}
		out = append(out, session)
	}
	return out
}

func (h *controlWSHub) sessionsByUser(userID string) []*controlWSSession {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]*controlWSSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		if session.userID != userID {
			continue
		}
		out = append(out, session)
	}
	return out
}

func (s *controlWSSession) send(msgType, requestID string, payload any) error {
	return s.sendTracked(msgType, requestID, payload, nil)
}

func (s *controlWSSession) sendTracked(
	msgType, requestID string,
	payload any,
	deps *routerDeps,
) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	err := websocket.JSON.Send(s.conn, controlws.Envelope{
		Type:      msgType,
		RequestID: requestID,
		Payload:   payload,
	})
	if err != nil {
		defaultControlWSHub.unregister(s.nodeID, s.conn)
		metricAdd("ws_send_error_total", 1)
		return err
	}
	return nil
}

func newControlWSInstanceID() string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "instance"
	}
	return hex.EncodeToString(buf[:])
}
