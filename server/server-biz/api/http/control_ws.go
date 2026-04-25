package httpapi

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

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
var controlWSDeliveryRetryOnce sync.Once
var defaultConnectPlanThrottle = newConnectPlanThrottle()
var defaultPeerCandidateWindow = newPeerCandidateWindow()

const (
	maxControlWSMessageBytes = 64 * 1024
	controlWSReadIdleTimeout = 90 * time.Second
)

func serveControlWS(conn *websocket.Conn, deps routerDeps) {
	defer conn.Close()
	conn.MaxPayloadBytes = maxControlWSMessageBytes
	metricAdd("ws_connection_total", 1)

	var session wsSession
	if request := conn.Request(); request != nil {
		session.deviceID = request.URL.Query().Get("deviceId")
		if session.deviceID == "" {
			session.deviceID = request.Header.Get("X-Slan-Device-Id")
		}
	}
	defer func() {
		defaultControlWSHub.unregister(session.nodeID)
		_ = deps.ControlChannel.CloseSession(session.userID, session.nodeID, session.networkID)
		fanoutPeerRemove(deps, session.networkID, session.nodeID)
	}()
	for {
		_ = conn.SetReadDeadline(time.Now().Add(controlWSReadIdleTimeout))
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

func startControlWSDeliveryRetryLoop(deps routerDeps) {
	if deps.MessageDelivery == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			messages, err := deps.MessageDelivery.ListRetryable(time.Now().Add(-3*time.Second).UnixMilli(), 64)
			if err != nil {
				continue
			}
			for _, message := range messages {
				if message.AttemptCount >= 5 {
					_ = deps.MessageDelivery.Archive(
						message,
						"undelivered",
						"http_ack_not_received_after_5_attempts",
						time.Now().UnixMilli(),
					)
					continue
				}
				session := defaultControlWSHub.session(message.TargetNodeID)
				if session != nil {
					_ = session.resendStored(message)
				}
				_ = deps.MessageDelivery.RecordAttempt(
					message.MessageID,
					message.AttemptCount+1,
					time.Now().UnixMilli(),
				)
			}
		}
	}()
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
