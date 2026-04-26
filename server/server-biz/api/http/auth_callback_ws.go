package httpapi

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"golang.org/x/net/websocket"
)

type authCallbackWSEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type authCallbackWSHub struct {
	mu       sync.RWMutex
	sessions map[string]map[*websocket.Conn]struct{}
}

func newAuthCallbackWSHub() *authCallbackWSHub {
	return &authCallbackWSHub{
		sessions: make(map[string]map[*websocket.Conn]struct{}),
	}
}

func (h *authCallbackWSHub) register(callbackID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[callbackID] == nil {
		h.sessions[callbackID] = make(map[*websocket.Conn]struct{})
	}
	h.sessions[callbackID][conn] = struct{}{}
}

func (h *authCallbackWSHub) unregister(callbackID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sessions := h.sessions[callbackID]
	if sessions == nil {
		return
	}
	delete(sessions, conn)
	if len(sessions) == 0 {
		delete(h.sessions, callbackID)
	}
}

func (h *authCallbackWSHub) broadcastReady(callbackID string, payload dto.CompleteAuthCallbackRequest) {
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.sessions[callbackID]))
	for conn := range h.sessions[callbackID] {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()

	for _, conn := range conns {
		if err := websocket.JSON.Send(conn, authCallbackWSEnvelope{
			Type:    "auth_callback_ready",
			Payload: mustMarshalRaw(payload),
		}); err != nil {
			h.unregister(callbackID, conn)
		}
	}
}

func mustMarshalRaw(payload any) json.RawMessage {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return encoded
}

var defaultAuthCallbackWSHub = newAuthCallbackWSHub()

const (
	maxAuthCallbackWSMessageBytes = 4 * 1024
	authCallbackWSReadIdleTimeout = 5 * time.Minute
)

func registerAuthCallbackWS(router *gin.Engine, deps routerDeps) {
	router.GET("/auth/ws/:callbackId", func(c *gin.Context) {
		callbackID := currentRouteContext(c).callbackID(c)
		websocket.Handler(func(conn *websocket.Conn) {
			defer conn.Close()
			conn.MaxPayloadBytes = maxAuthCallbackWSMessageBytes
			defaultAuthCallbackWSHub.register(callbackID, conn)
			defer defaultAuthCallbackWSHub.unregister(callbackID, conn)

			if status, err := deps.Auth.GetCallbackStatus(callbackID); err == nil && status.Ready && status.Payload != nil {
				_ = websocket.JSON.Send(conn, authCallbackWSEnvelope{
					Type:    "auth_callback_ready",
					Payload: mustMarshalRaw(*status.Payload),
				})
			}

			for {
				_ = conn.SetReadDeadline(time.Now().Add(authCallbackWSReadIdleTimeout))
				var env authCallbackWSEnvelope
				if err := websocket.JSON.Receive(conn, &env); err != nil {
					return
				}
			}
		}).ServeHTTP(c.Writer, c.Request)
	})
}
