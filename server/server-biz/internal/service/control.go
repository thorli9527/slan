package service

import (
	"time"

	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
)

// Bootstrap defines client bootstrap and relay-fallback capabilities.
//
// Bootstrapping is the "one request to start the client" API:
// - It validates that the caller owns the node/device and that the device is an active member of the network.
// - It returns initial topology (NetworkMap), control-plane configuration, relay/DERP configuration, and device attachments.
//
// Implementations should keep the response self-contained so clients do not need extra calls during startup.
type Bootstrap interface {
	// CreateControlSession 为指定节点创建控制面会话，并返回控制面连接参数与初始网络图。
	//
	// Use case:
	// - The client already has device/node/network context and only needs to refresh the control-plane session token
	//   and its derived networkMap snapshot.
	//
	// Typical errors:
	// - ErrForbidden: node does not belong to the user or is not allowed in the network.
	// - ErrNotFound: node or network not found.
	CreateControlSession(userID string, req dto.CreateControlSessionRequest) (dto.ControlSessionResponse, error)
	// Bootstrap 返回客户端启动时所需的完整基础配置，包括设备、网络、控制面和中继视图。
	//
	// Use case:
	// - The preferred public client flow. Returns everything needed to start:
	//   - device bootstrap view (device + attachments)
	//   - networks visible to the device
	//   - control-plane config/sessionToken
	//   - STUN/relay/DERP topology
	//   - initial network map
	//
	// Typical errors:
	// - ErrForbidden: device not an active member, missing active attachment/virtual IP, or node ownership mismatch.
	// - ErrNotFound: node/network/device not found.
	Bootstrap(userID string, req dto.BootstrapRequest) (dto.BootstrapResponse, error)
	// IssueRelayTicket 为源节点到目标节点的回退中继路径签发可验证的 relay ticket。
	//
	// Use case:
	// - When direct P2P paths fail, the client requests a short-lived relay ticket to attach to server-relay.
	//
	// Security:
	// - The issuer must validate that src/dst nodes are in the same network and the caller is allowed to connect them.
	//
	// Typical errors:
	// - ErrForbidden: src/dst not in same network, or caller lacks access.
	IssueRelayTicket(userID string, req dto.RelayTicketRequest) (dto.RelayTicket, error)
}

// ControlChannel abstracts the control channel handshake and topology/state reporting.
//
// Transport:
// - In this repository, the control plane is primarily delivered over MQTT topics:
//   - device publishes control/up; server publishes control/down.
//
// - HTTP endpoints are used to create sessions and bootstrap configuration.
//
// Semantics:
// - Handshake establishes an authenticated session (NodeHello -> Ack) and returns an initial NetworkMap snapshot.
// - NetworkMap reports the latest topology snapshot for a node within a network.
// - Report* methods accept runtime signals that feed back into topology updates and connect plan generation.
type ControlChannel interface {
	// Handshake 校验控制面会话并返回握手确认以及当前网络地图快照。
	//
	// Input:
	// - hello contains user/device/node/network context and the control session token.
	//
	// Output:
	// - NodeHelloAck acknowledges the session.
	// - dto.NetworkMap is the latest snapshot after the handshake.
	//
	// Typical errors:
	// - ErrUnauthorized: session token invalid/expired.
	// - ErrForbidden: node/device not allowed for the network.
	Handshake(hello controlmsg.NodeHello) (controlmsg.NodeHelloAck, dto.NetworkMap, error)
	// NetworkMap 返回当前节点在指定网络内可见的完整拓扑快照。
	//
	// Typical errors:
	// - ErrForbidden: caller not allowed to view the network/node.
	NetworkMap(userID, nodeID, networkID string) (dto.NetworkMap, error)
	// ReportEndpoints 上报节点当前可达端点和 NAT 观测，并返回刷新后的网络地图。
	//
	// Use case:
	// - The client sends LAN/WAN endpoints; the server updates peer candidates and may trigger connect plan fanout.
	ReportEndpoints(userID string, report controlmsg.EndpointReport) (dto.NetworkMap, error)
	// ReportConnectionState 上报当前节点到某个对端节点的连接状态变化。
	//
	// Use case:
	// - Clients report connecting/connected/failed/closed so the server can update health and decide retries/fallback.
	ReportConnectionState(userID, nodeID string, state controlmsg.ConnectionState) error
	// ReportPathHealth 上报某条 direct 或 relay 路径的质量观测结果。
	//
	// Use case:
	// - Used for quality scoring and relay policy decisions.
	ReportPathHealth(userID, nodeID string, report controlmsg.PathHealthReport) error
	// ReportRelayHeartbeat records one relay node heartbeat from MQTT.
	//
	// Caller:
	// - relay daemons publish heartbeat; server-biz records them for relay topology/health.
	ReportRelayHeartbeat(report controlmsg.RelayNodeHeartbeat) error
	// ReportRelayPolicy records client-side relay data-plane policy execution.
	//
	// Use case:
	// - The client reports which relay path it picked and the observed policy outcome.
	ReportRelayPolicy(userID, nodeID string, report controlmsg.RelayPolicyReport) error
	// Disconnect 显式关闭当前节点到某个对端节点的连接关系。
	//
	// Use case:
	// - When a client voluntarily disconnects, the server can clean up topology and notify peers.
	Disconnect(userID, nodeID string, notice controlmsg.DisconnectNotice) error
	// Heartbeat 刷新指定节点控制面会话的活跃时间，避免其被当作离线节点清理。
	//
	// Notes:
	// - Called on each ping and on other control messages as a "liveness" signal.
	Heartbeat(userID, nodeID, networkID string) error
	// CloseSession 关闭节点在某个网络下的控制面会话并清理其瞬态状态。
	//
	// Use case:
	// - Used when switching networks or when the device is disabled to force a clean shutdown.
	CloseSession(userID, nodeID, networkID string) error
	// PeerSnapshot 返回当前节点视角下某个对端节点的拓扑快照。
	//
	// Use case:
	// - On-demand queries for diagnostics or targeted connect plan generation.
	PeerSnapshot(userID, nodeID, networkID, peerNodeID string) (dto.Peer, error)
	// ConnectPlan 生成当前节点到指定对端节点的路径建议和 relay 回退信息。
	//
	// Output:
	// - controlmsg.ConnectPlan containing path candidates and optional relay ticket instructions.
	ConnectPlan(userID, nodeID, networkID, peerNodeID string) (controlmsg.ConnectPlan, error)
	// ConnectPlanByNode 在只知道源节点 ID 的场景下生成对应的连接规划。
	//
	// Use case:
	// - Used by server-side fanout when the caller context already includes a session and only node IDs are needed.
	ConnectPlanByNode(nodeID, networkID, peerNodeID string) (controlmsg.ConnectPlan, error)
	// ActiveSessions returns fresh control sessions in a network for MQTT fanout.
	//
	// Input:
	// - excludeNodeID allows omitting the caller from broadcasts.
	ActiveSessions(networkID, excludeNodeID string) ([]ControlSession, error)
	// LatestSessionByDevice returns the newest fresh control session for a device.
	//
	// Use case:
	// - When the server only knows deviceID (e.g., derived from MQTT topic) and needs to hydrate node/network context.
	LatestSessionByDevice(deviceID string) (ControlSession, error)
}

// ControlSession is the minimal session identity required to address a device/node over the control plane.
type ControlSession struct {
	UserID    string
	DeviceID  string
	NodeID    string
	NetworkID string
}

// ControlSync abstracts cross-instance synchronization for control-plane events.
//
// In multi-instance deployments, one server-biz instance may handle an incoming control/up message,
// but needs to notify other instances to fanout to their connected devices. Implementations usually
// rely on a shared pubsub backend.
type ControlSync interface {
	// Publish 向跨实例同步通道发布一个控制面事件。
	//
	// Typical events:
	// - peer_update / peer_remove / connect_plan fanout triggers
	Publish(event controlmsg.ControlSyncEvent) error
	// Subscribe 订阅跨实例控制面事件，并对每个事件执行回调处理。
	//
	// Notes:
	// - Subscribe is typically a long-running call that reconnects internally on backend failures.
	Subscribe(handler func(controlmsg.ControlSyncEvent)) error
	// NextRevision 递增并返回指定网络的控制面修订号。
	//
	// Use case:
	// - Revision increments when topology changes; clients use revision to decide whether to apply updates.
	NextRevision(networkID string) (uint64, error)
	// CurrentRevision 返回指定网络当前的控制面修订号，不做递增。
	CurrentRevision(networkID string) (uint64, error)
	// AcquireConnectPlanRetry 获取某个节点对的 connect plan 重试窗口，避免短时间重复生成。
	//
	// Return:
	// - allowed=true means the caller can retry generating a connect plan for this pair now.
	AcquireConnectPlanRetry(networkID, nodeID, peerNodeID string) (bool, error)
	// ResetConnectPlanRetry 清除某个节点对的 connect plan 重试状态。
	ResetConnectPlanRetry(networkID, nodeID, peerNodeID string) error
	// AcquirePeerCandidateDelivery 获取 peer candidate 的去重投递窗口，避免重复转发。
	//
	// Inputs:
	// - candidate and ttl form the deduplication key/window.
	AcquirePeerCandidateDelivery(networkID, sourceNodeID, targetNodeID string, candidate controlmsg.PeerCandidate, ttl time.Duration) (bool, error)
}
