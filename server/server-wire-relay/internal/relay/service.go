package relay

import (
	"net"

	"github.com/slan/server/server-wire-relay/internal/protocol"
	"github.com/slan/server/server-wire-relay/internal/state"
)

// Service 承载 relay_udp 数据面的会话绑定、转发和释放业务。
// UDPServer 负责 UDP 协议解析和写包，Service 负责调用状态仓储完成业务动作。
type Service struct {
	store state.PacketStore
}

func NewService(store state.PacketStore) *Service {
	return &Service{store: store}
}

func (s *Service) RefreshParticipant(addr *net.UDPAddr, sessionID, participantID string) error {
	return s.store.RefreshParticipant(addr, sessionID, participantID)
}

func (s *Service) Attach(addr *net.UDPAddr, participantID string, ticket protocol.RelayTicket, transport string) (string, error) {
	_, peerID, err := s.store.Attach(addr, participantID, ticket, transport)
	return peerID, err
}

func (s *Service) Forward(addr *net.UDPAddr, sessionID, participantID string, payload []byte) (*net.UDPAddr, string, error) {
	return s.store.Forward(addr, sessionID, participantID, payload)
}

func (s *Service) Detach(addr *net.UDPAddr, sessionID, participantID string) error {
	return s.store.Detach(addr, sessionID, participantID)
}
