package derp

import (
	"net"

	"github.com/slan/server/server-wire-derp/internal/protocol"
	"github.com/slan/server/server-wire-derp/internal/state"
)

// Service 承载 DERP TCP 数据面的连接、会话绑定和断开业务。
// Server 负责 TCP 协议循环和 writer 管理，Service 负责状态仓储编排。
type Service struct {
	store state.StreamStore
}

func NewService(store state.StreamStore) *Service {
	return &Service{store: store}
}

func (s *Service) Connect(conn net.Conn, peerID, nodeID, regionID string, ticket protocol.DerpTicket) (state.Session, int64, error) {
	return s.store.Connect(conn, peerID, nodeID, regionID, ticket)
}

func (s *Service) TouchPeer(peerID string) {
	s.store.TouchPeer(peerID)
}

func (s *Service) BindSessionPeer(sessionID, currentPeerID, targetPeerID string) (state.Session, error) {
	return s.store.BindSessionPeer(sessionID, currentPeerID, targetPeerID)
}

func (s *Service) Disconnect(peerID string) {
	s.store.Disconnect(peerID)
}
