package service

import (
	"github.com/slan/server/server-biz/api/dto"
	controlws "github.com/slan/server/server-biz/internal/ws"
	"time"
)

// Bootstrap 定义客户端启动配置和中继回退能力。
type Bootstrap interface {
	// CreateControlSession 为指定节点创建控制面会话，并返回控制面连接参数与初始网络图。
	CreateControlSession(userID string, req dto.CreateControlSessionRequest) (dto.ControlSessionResponse, error)
	// Bootstrap 返回客户端启动时所需的完整基础配置，包括设备、网络、控制面和中继视图。
	Bootstrap(userID string, req dto.BootstrapRequest) (dto.BootstrapResponse, error)
	// IssueRelayTicket 为源节点到目标节点的回退中继路径签发可验证的 relay ticket。
	IssueRelayTicket(userID string, req dto.RelayTicketRequest) (dto.RelayTicket, error)
}

// ControlChannel 抽象了控制通道建立和网络地图读取能力。
type ControlChannel interface {
	// Handshake 校验控制面会话并返回握手确认以及当前网络地图快照。
	Handshake(hello controlws.NodeHello) (controlws.NodeHelloAck, dto.NetworkMap, error)
	// NetworkMap 返回当前节点在指定网络内可见的完整拓扑快照。
	NetworkMap(userID, nodeID, networkID string) (dto.NetworkMap, error)
	// ReportEndpoints 上报节点当前可达端点和 NAT 观测，并返回刷新后的网络地图。
	ReportEndpoints(userID string, report controlws.EndpointReport) (dto.NetworkMap, error)
	// ReportConnectionState 上报当前节点到某个对端节点的连接状态变化。
	ReportConnectionState(userID, nodeID string, state controlws.ConnectionState) error
	// ReportPathHealth 上报某条 direct 或 relay 路径的质量观测结果。
	ReportPathHealth(userID, nodeID string, report controlws.PathHealthReport) error
	// Disconnect 显式关闭当前节点到某个对端节点的连接关系。
	Disconnect(userID, nodeID string, notice controlws.DisconnectNotice) error
	// Heartbeat 刷新指定节点控制面会话的活跃时间，避免其被当作离线节点清理。
	Heartbeat(userID, nodeID, networkID string) error
	// CloseSession 关闭节点在某个网络下的控制面会话并清理其瞬态状态。
	CloseSession(userID, nodeID, networkID string) error
	// PeerSnapshot 返回当前节点视角下某个对端节点的拓扑快照。
	PeerSnapshot(userID, nodeID, networkID, peerNodeID string) (dto.Peer, error)
	// ConnectPlan 生成当前节点到指定对端节点的路径建议和 relay 回退信息。
	ConnectPlan(userID, nodeID, networkID, peerNodeID string) (controlws.ConnectPlan, error)
	// ConnectPlanByNode 在只知道源节点 ID 的场景下生成对应的连接规划。
	ConnectPlanByNode(nodeID, networkID, peerNodeID string) (controlws.ConnectPlan, error)
	// ActiveSessions returns fresh control sessions in a network for MQTT fanout.
	ActiveSessions(networkID, excludeNodeID string) ([]ControlSession, error)
}

type ControlSession struct {
	UserID    string
	DeviceID  string
	NodeID    string
	NetworkID string
}

// ControlSync 抽象了多实例控制通道事件同步。
type ControlSync interface {
	// Publish 向跨实例同步通道发布一个控制面事件。
	Publish(event controlws.ControlSyncEvent) error
	// Subscribe 订阅跨实例控制面事件，并对每个事件执行回调处理。
	Subscribe(handler func(controlws.ControlSyncEvent)) error
	// NextRevision 递增并返回指定网络的控制面修订号。
	NextRevision(networkID string) (uint64, error)
	// CurrentRevision 返回指定网络当前的控制面修订号，不做递增。
	CurrentRevision(networkID string) (uint64, error)
	// AcquireConnectPlanRetry 获取某个节点对的 connect plan 重试窗口，避免短时间重复生成。
	AcquireConnectPlanRetry(networkID, nodeID, peerNodeID string) (bool, error)
	// ResetConnectPlanRetry 清除某个节点对的 connect plan 重试状态。
	ResetConnectPlanRetry(networkID, nodeID, peerNodeID string) error
	// AcquirePeerCandidateDelivery 获取 peer candidate 的去重投递窗口，避免重复转发。
	AcquirePeerCandidateDelivery(networkID, sourceNodeID, targetNodeID string, candidate controlws.PeerCandidate, ttl time.Duration) (bool, error)
}
