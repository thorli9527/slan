package state

import (
	"net"

	"github.com/slan/server/server-wire-derp/internal/protocol"
)

// StoreAPI 是 DERP TCP 数据面和管理 HTTP 入口共享的状态抽象。
// Store 是当前默认内存实现。
type StoreAPI interface {
	StreamStore
	AdminViewStore
}

// StreamStore 定义 DERP TCP 数据面处理连接、发包和断开所需的方法。
type StreamStore interface {
	// Connect 校验 DERP ticket，登记 peer 连接，并创建服务端会话。
	Connect(conn net.Conn, peerID, nodeID, regionID string, ticket protocol.DerpTicket) (Session, int64, error)
	// TouchPeer 刷新 peer 最近活跃时间。
	TouchPeer(peerID string)
	// BindSessionPeer 校验当前 peer 属于会话，并把会话绑定到目标 peer。
	BindSessionPeer(sessionID, currentPeerID, targetPeerID string) (Session, error)
	// Disconnect 移除 peer 连接以及关联会话。
	Disconnect(peerID string)
}

// AdminViewStore 定义 DERP 管理 HTTP 接口查看运行状态所需的方法。
type AdminViewStore interface {
	// Connections 返回当前 peer 连接列表。
	Connections() []ConnectionView
	// Connection 按 peer ID 查询单个连接视图。
	Connection(peerID string) (ConnectionView, bool)
	// Sessions 返回当前 DERP 会话列表。
	Sessions() []SessionView
	// Session 按会话 ID 查询单个 DERP 会话视图。
	Session(sessionID string) (SessionView, bool)
	// Regions 返回按 region 聚合后的健康视图。
	Regions() []RegionView
	// Metrics 返回 DERP 服务运行指标。
	Metrics() Metrics
}

var _ StoreAPI = (*Store)(nil)
