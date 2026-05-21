package state

import (
	"net"

	"github.com/slan/server/server-wire-relay/internal/protocol"
)

// StoreAPI 是中继 UDP 数据面和管理 HTTP 入口共享的状态抽象。
// Store 是当前默认内存实现。
type StoreAPI interface {
	PacketStore
	AdminViewStore
}

// PacketStore 定义 UDP 中继数据面需要的会话绑定、转发和释放能力。
type PacketStore interface {
	// Attach 校验 relay ticket，并把参与方 UDP 源地址绑定到中继会话。
	Attach(addr *net.UDPAddr, participantID string, ticket protocol.RelayTicket, transport string) (*Session, string, error)
	// RefreshParticipant 刷新参与方地址，用于 keepalive 或地址漂移后的重绑定。
	RefreshParticipant(addr *net.UDPAddr, sessionID, participantID string) error
	// Forward 查询对端地址并记录转发计数，调用方负责实际写 UDP 包。
	Forward(addr *net.UDPAddr, sessionID, participantID string, payload []byte) (*net.UDPAddr, string, error)
	// Detach 从中继会话中移除参与方，最后一个参与方离开时会话可被释放。
	Detach(addr *net.UDPAddr, sessionID, participantID string) error
}

// AdminViewStore 定义管理 HTTP 接口只读查看运行状态所需的方法。
type AdminViewStore interface {
	// Sessions 返回当前中继会话列表。
	Sessions() []SessionView
	// Session 按 ID 查询单个中继会话视图。
	Session(sessionID string) (SessionView, bool)
	// Metrics 返回中继服务运行指标。
	Metrics() Metrics
}

var _ StoreAPI = (*Store)(nil)
