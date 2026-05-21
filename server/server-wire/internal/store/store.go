package store

import (
	"time"

	"github.com/slan/server/server-wire/internal/model"
)

// Store 定义 server-wire 对 peer 状态、路径健康、DERP map 和 relay/DERP
// 票据签发的持久化边界。
type Store interface {
	// RegisterPeer 注册或刷新 peer 能力声明。
	RegisterPeer(model.PeerRegistration) (model.PeerRecord, error)
	// UpdateEndpoints 更新 peer 当前可用端点候选。
	UpdateEndpoints(peerID string, endpoints []model.Endpoint) (model.PeerRecord, error)
	// UpdatePathHealth 更新 peer 对各数据路径的探测结果。
	UpdatePathHealth(peerID string, probes []model.PathProbe) (model.PeerRecord, error)
	// UpdateDerpHealth 更新 peer 对 DERP 节点的探测结果。
	UpdateDerpHealth(peerID string, samples []model.DerpHealthSample) (model.PeerRecord, error)
	// UpdateActivePath 记录 peer 当前实际使用的数据路径。
	UpdateActivePath(peerID string, path model.PathKind) (model.PeerRecord, error)
	// IssueRelayTicket 为 peer 签发连接指定 UDP relay 的短期票据。
	IssueRelayTicket(peerID string, relay model.RelayNode, ttl time.Duration, renewAfter time.Duration) (model.RelayTicket, error)
	// IssueDerpTicket 为 peer 签发连接指定 DERP 节点的短期票据。
	IssueDerpTicket(peerID, regionID, nodeID string, ttl time.Duration, renewAfter time.Duration) (model.DerpTicket, error)
	// DerpMap 返回当前可见的 DERP 节点地图。
	DerpMap() model.DerpMap
	// GetPeer 按 peer ID 查询本地记录。
	GetPeer(peerID string) (model.PeerRecord, bool)
	// ListPeersByNetwork 查询同一虚拟网络内的 peer 列表。
	ListPeersByNetwork(networkID string) []model.PeerRecord
}
