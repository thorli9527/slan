package httpapi

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/slan/server/server-biz/internal/service"
	controlws "github.com/slan/server/server-biz/internal/ws"
	"golang.org/x/net/websocket"
)

// wsEnvelope 是 HTTP/WebSocket 层解码入站消息时使用的轻量封装。
type wsEnvelope struct {
	// Type 标识消息负载类型。
	Type string `json:"type"`
	// RequestID 用于在请求/响应型消息中回传关联 ID。
	RequestID string `json:"requestId,omitempty"`
	// Payload 是原始 JSON 负载，后续按 Type 解码。
	Payload json.RawMessage `json:"payload,omitempty"`
}

var defaultControlWSHub = newControlWSHub()
var controlWSInstanceID = newControlWSInstanceID()
var controlWSSyncOnce sync.Once
var defaultConnectPlanThrottle = newConnectPlanThrottle()
var defaultPeerCandidateWindow = newPeerCandidateWindow()

func serveControlWS(conn *websocket.Conn, deps routerDeps) {
	defer conn.Close()
	metricAdd("ws_connection_total", 1)

	var session wsSession
	defer func() {
		defaultControlWSHub.unregister(session.nodeID)
		_ = deps.ControlChannel.CloseSession(session.userID, session.nodeID, session.networkID)
		fanoutPeerRemove(deps, session.networkID, session.nodeID)
	}()
	for {
		var env wsEnvelope
		if err := websocket.JSON.Receive(conn, &env); err != nil {
			metricAdd("ws_receive_error_total", 1)
			return
		}
		metricAdd("ws_message_received_total", 1)
		metricAddByType(controlWSMessageTypeMetrics, env.Type, 1)
		if err := handleControlWSMessage(conn, deps, &session, env); err != nil {
			return
		}
	}
}

func writeWSEnvelope(conn *websocket.Conn, msgType, requestID string, payload any) error {
	return websocket.JSON.Send(conn, controlws.Envelope{
		Type:      msgType,
		RequestID: requestID,
		Payload:   payload,
	})
}

func writeWSError(conn *websocket.Conn, requestID string, err error) {
	code := "INTERNAL"
	switch {
	case errors.Is(err, service.ErrInvalidArgument):
		code = "INVALID_ARGUMENT"
	case errors.Is(err, service.ErrUnauthorized):
		code = "UNAUTHORIZED"
	case errors.Is(err, service.ErrForbidden):
		code = "FORBIDDEN"
	case errors.Is(err, service.ErrNotFound):
		code = "NOT_FOUND"
	case errors.Is(err, service.ErrConflict):
		code = "CONFLICT"
	}
	metricAddByType(controlWSErrorCodeMetrics, code, 1)
	_ = writeWSEnvelope(conn, "error", requestID, controlws.ErrorMessage{
		Code:    code,
		Message: err.Error(),
	})
}
