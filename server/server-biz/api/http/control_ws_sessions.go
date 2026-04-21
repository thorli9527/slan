package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
	controlws "github.com/slan/server/server-biz/internal/ws"
	"golang.org/x/net/websocket"
)

// wsSession 是控制通道 handler 共享的轻量会话视图。
//
// 它只保存业务鉴权后真正需要的标识，不直接持有连接对象，
// 便于 handler、fanout 和 metrics 在不暴露底层连接细节的前提下协作。
type wsSession struct {
	userID    string
	deviceID  string
	nodeID    string
	networkID string
}

// controlWSSession 是挂在 hub 中的活跃 WebSocket 会话。
//
// 相比 wsSession，它额外持有：
// - websocket 连接
// - 写锁
// 用于安全地向特定节点发送消息。
type controlWSSession struct {
	conn      *websocket.Conn
	writeMu   sync.Mutex
	userID    string
	deviceID  string
	nodeID    string
	networkID string
}

// controlWSHub 管理当前进程内所有活跃的 control WS 会话。
type controlWSHub struct {
	mu       sync.RWMutex
	sessions map[string]*controlWSSession
}

// newControlWSHub 创建一个空的进程内 WS hub。
func newControlWSHub() *controlWSHub {
	return &controlWSHub{
		sessions: make(map[string]*controlWSSession),
	}
}

// register 用 nodeID 注册一个活跃会话，并同步更新 metrics。
func (h *controlWSHub) register(session *controlWSSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[session.nodeID] = session
	controlWSActiveSessions.Set(int64(len(h.sessions)))
	metricAdd("session_register_total", 1)
}

// unregister 按 nodeID 移除活跃会话。
func (h *controlWSHub) unregister(nodeID string) {
	if nodeID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, nodeID)
	controlWSActiveSessions.Set(int64(len(h.sessions)))
	metricAdd("session_unregister_total", 1)
}

// session 返回某个 nodeID 当前对应的活跃连接。
func (h *controlWSHub) session(nodeID string) *controlWSSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[nodeID]
}

// peersInNetwork 返回同一网络下除自身外的所有活跃会话。
//
// 这个结果主要用于 peer update / remove 的本机 fanout。
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

// sessionsByUser 返回同一用户当前所有活跃会话。
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

// send 以串行写方式向该连接发送一条标准 Envelope 消息。
func (s *controlWSSession) send(msgType, requestID string, payload any) error {
	return s.sendTracked(msgType, requestID, payload, nil)
}

func (s *controlWSSession) sendTracked(
	msgType, requestID string,
	payload any,
	deps *routerDeps,
) error {
	messageID := ""
	if deps != nil && deps.MessageDelivery != nil {
		payloadJSON, err := json.Marshal(payload)
		if err == nil {
			messageID = util.NewID("msg")
			now := time.Now().UnixMilli()
			_ = deps.MessageDelivery.CreatePending(repo.ControlOutboundMessage{
				MessageID:     messageID,
				TargetUserID:  s.userID,
				TargetNodeID:  s.nodeID,
				NetworkID:     s.networkID,
				MessageType:   msgType,
				RequestID:     requestID,
				PayloadJSON:   string(payloadJSON),
				AttemptCount:  1,
				Status:        "pending",
				LastAttemptAt: now,
				CreatedAt:     now,
				UpdatedAt:     now,
			})
		}
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return websocket.JSON.Send(s.conn, controlws.Envelope{
		Type:      msgType,
		RequestID: requestID,
		MessageID: messageID,
		Payload:   payload,
	})
}

func (s *controlWSSession) resendStored(message repo.ControlOutboundMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return websocket.JSON.Send(s.conn, controlws.Envelope{
		Type:      message.MessageType,
		RequestID: message.RequestID,
		MessageID: message.MessageID,
		Payload:   json.RawMessage(message.PayloadJSON),
	})
}

// newControlWSInstanceID 生成当前 server-biz 进程实例 ID。
//
// 它用于多实例 ControlSync 里标记事件来源，避免回环处理。
func newControlWSInstanceID() string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "instance"
	}
	return hex.EncodeToString(buf[:])
}
