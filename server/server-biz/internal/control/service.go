package control

import "github.com/slan/server/server-biz/api/dto"

// BootstrapService 定义客户端启动配置和中继回退能力。
type BootstrapService interface {
	// CreateControlSession 创建控制通道会话并返回初始网络地图。
	CreateControlSession(userID string, req dto.CreateControlSessionRequest) (dto.ControlSessionResponse, error)
	// Bootstrap 返回设备、挂载、网络、STUN、控制通道和中继配置。
	Bootstrap(userID string, req dto.BootstrapRequest) (dto.BootstrapResponse, error)
	// IssueRelayTicket 在 P2P 失败时返回短时效的中继票据。
	IssueRelayTicket(userID string, req dto.RelayTicketRequest) (dto.RelayTicket, error)
}
