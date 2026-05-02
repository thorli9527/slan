package repo

import (
	"github.com/lib/pq"
	"github.com/slan/server/server-biz/api/dto"
)

// Node 是可参与组网通信的逻辑节点持久化模型。
type Node struct {
	// NodeID 是节点唯一标识。
	NodeID string `gorm:"column:node_id;primaryKey"`
	// UserID 是节点所属用户。
	UserID string `gorm:"column:user_id;index;not null"`
	// DeviceID 是节点所属设备。
	DeviceID string `gorm:"column:device_id;index;not null"`
	// NodePublicKey 是节点公钥。
	NodePublicKey string `gorm:"column:node_public_key;not null"`
	// Capabilities 是节点能力集合。
	Capabilities pq.StringArray `gorm:"column:capabilities;type:text[];not null"`
}

func (Node) TableName() string { return "nodes" }

// NodeEndpoint 记录节点在某个网络下最近上报的可达端点。
type NodeEndpoint struct {
	// EndpointID 是端点记录唯一标识。
	EndpointID string `gorm:"column:endpoint_id;primaryKey"`
	// NodeID 是端点所属节点。
	NodeID string `gorm:"column:node_id;index;not null"`
	// NetworkID 是端点所属网络。
	NetworkID string `gorm:"column:network_id;index;not null"`
	// NatType 是观测到的 NAT 类型。
	NatType string `gorm:"column:nat_type;not null;default:''"`
	// Type 是端点类型。
	Type string `gorm:"column:type;not null"`
	// Address 是候选地址。
	Address string `gorm:"column:address;not null"`
	// UpdatedAt 是上报时间。
	UpdatedAt int64 `gorm:"column:updated_at;not null"`
}

func (NodeEndpoint) TableName() string { return "node_endpoints" }

// NodeConnectionState 记录一个节点对最近一次连接状态上报。
type NodeConnectionState struct {
	// StateID 是状态记录唯一标识。
	StateID string `gorm:"column:state_id;primaryKey"`
	// NetworkID 是所属网络。
	NetworkID string `gorm:"column:network_id;index;not null;uniqueIndex:idx_node_peer_state"`
	// NodeID 是源节点。
	NodeID string `gorm:"column:node_id;index;not null;uniqueIndex:idx_node_peer_state"`
	// PeerNodeID 是目标节点。
	PeerNodeID string `gorm:"column:peer_node_id;index;not null;uniqueIndex:idx_node_peer_state"`
	// Path 是当前路径类型。
	Path string `gorm:"column:path;not null;default:''"`
	// State 是连接状态值。
	State string `gorm:"column:state;not null"`
	// Reason 是状态原因。
	Reason string `gorm:"column:reason;not null;default:''"`
	// ObservedRttMs 是 RTT 观测。
	ObservedRttMs *uint32 `gorm:"column:observed_rtt_ms"`
	// PacketLossPpm 是丢包率观测。
	PacketLossPpm *uint32 `gorm:"column:packet_loss_ppm"`
	// PathScore 是本地路径评分。
	PathScore *uint32 `gorm:"column:path_score"`
	// DerpNodeID 是关联的 relay/DERP 节点。
	DerpNodeID string `gorm:"column:derp_node_id;not null;default:''"`
	// UpdatedAt 是最近上报时间。
	UpdatedAt int64 `gorm:"column:updated_at;not null"`
}

func (NodeConnectionState) TableName() string { return "node_connection_states" }

// NodePathHealth 记录一个节点对某条路径最近一次聚合健康样本。
type NodePathHealth struct {
	// HealthID 是样本记录唯一标识。
	HealthID string `gorm:"column:health_id;primaryKey"`
	// NetworkID 是所属网络。
	NetworkID string `gorm:"column:network_id;index;not null;uniqueIndex:idx_node_peer_path_health"`
	// NodeID 是源节点。
	NodeID string `gorm:"column:node_id;index;not null;uniqueIndex:idx_node_peer_path_health"`
	// PeerNodeID 是目标节点。relay 节点全局质量样本允许为空。
	PeerNodeID string `gorm:"column:peer_node_id;index;not null;default:'';uniqueIndex:idx_node_peer_path_health"`
	// PathType 是路径类型。
	PathType string `gorm:"column:path_type;not null;uniqueIndex:idx_node_peer_path_health"`
	// Endpoint 是对应端点地址。
	Endpoint string `gorm:"column:endpoint;not null;default:'';uniqueIndex:idx_node_peer_path_health"`
	// DerpNodeID 是对应 relay/DERP 节点。
	DerpNodeID string `gorm:"column:derp_node_id;not null;default:'';uniqueIndex:idx_node_peer_path_health"`
	// ObservedRttMs 是 RTT 观测。
	ObservedRttMs *uint32 `gorm:"column:observed_rtt_ms"`
	// PacketLossPpm 是丢包率观测。
	PacketLossPpm *uint32 `gorm:"column:packet_loss_ppm"`
	// PathScore 是路径评分。
	PathScore *uint32 `gorm:"column:path_score"`
	// SampledAtMs 是样本采集时间。
	SampledAtMs uint64 `gorm:"column:sampled_at_ms;not null;default:0"`
	// UpdatedAt 是样本写入时间。
	UpdatedAt int64 `gorm:"column:updated_at;not null"`
}

func (NodePathHealth) TableName() string { return "node_path_health" }

// RelayNodeHeartbeat records the latest MQTT heartbeat published by one relay node.
type RelayNodeHeartbeat struct {
	NodeID         string `gorm:"column:node_id;primaryKey"`
	ClusterID      string `gorm:"column:cluster_id;index;not null;default:''"`
	CountryCode    string `gorm:"column:country_code;index;not null;default:''"`
	CityCode       string `gorm:"column:city_code;index;not null;default:''"`
	Transport      string `gorm:"column:transport;not null;default:''"`
	Address        string `gorm:"column:address;not null;default:''"`
	Healthy        bool   `gorm:"column:healthy;not null;default:true"`
	ActiveSessions int    `gorm:"column:active_sessions;not null;default:0"`
	ReportedAtMs   uint64 `gorm:"column:reported_at_ms;not null;default:0"`
	UpdatedAt      int64  `gorm:"column:updated_at;index;not null"`
}

func (RelayNodeHeartbeat) TableName() string { return "relay_node_heartbeats" }

func (m Node) ToDTO(networkIDs []string) dto.Node {
	return dto.Node{
		NodeID:        m.NodeID,
		DeviceID:      m.DeviceID,
		NodePublicKey: m.NodePublicKey,
		NetworkIDs:    networkIDs,
		Capabilities:  append([]string(nil), m.Capabilities...),
	}
}

func (m NodeEndpoint) ToDTO() dto.Endpoint {
	return dto.Endpoint{
		Type:      m.Type,
		Address:   m.Address,
		UpdatedAt: m.UpdatedAt,
	}
}
