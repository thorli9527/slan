package controlmsg

// EndpointReport 用于节点向控制面上报本地端点和 NAT 观测结果。
type EndpointReport struct {
	// NetworkID 是所属网络。
	NetworkID string `json:"networkId"`
	// NodeID 是当前节点 ID。
	NodeID string `json:"nodeId"`
	// NatType 是观测到的 NAT 类型。
	NatType string `json:"natType"`
	// Endpoints 是本地可用候选端点列表。
	Endpoints []Endpoint `json:"endpoints,omitempty"`
}

// PeerCandidate 承载对等端之间交换的候选路径信息。
type PeerCandidate struct {
	// PeerNodeID 是目标远端节点 ID。
	PeerNodeID string `json:"peerNodeId"`
	// CandidateType 描述候选来源，例如 reflexive 或 relay。
	CandidateType string `json:"candidateType"`
	// Foundation 是 ICE-like foundation。
	Foundation string `json:"foundation,omitempty"`
	// Component 是组件类型。
	Component int `json:"component,omitempty"`
	// Protocol 是传输协议，例如 udp。
	Protocol string `json:"protocol,omitempty"`
	// Address 是候选地址。
	Address string `json:"address,omitempty"`
	// Port 是候选端口。
	Port int `json:"port,omitempty"`
	// Priority 是候选优先级。
	Priority int `json:"priority,omitempty"`
	// RelatedAddress 是关联地址。
	RelatedAddress string `json:"relatedAddress,omitempty"`
	// RelatedPort 是关联端口。
	RelatedPort int `json:"relatedPort,omitempty"`
	// RelayURL 是 relay 候选对应的 relay URL。
	RelayURL string `json:"relayUrl,omitempty"`
	// Endpoint 是用于直连尝试的网络地址。
	Endpoint string `json:"endpoint"`
	// Priority 是候选优先级，数值越小优先级越高。
	PriorityRank int `json:"priority,omitempty"`
}

// PathHealthReport 用于节点向控制面上报路径健康度采样。
type PathHealthReport struct {
	// NetworkID 是所属网络。
	NetworkID string `json:"networkId"`
	// PeerNodeID 是对端节点 ID。
	PeerNodeID string `json:"peerNodeId"`
	// PathType 是路径类型，例如 lan / wan / reflexive / relay / derp。
	PathType string `json:"pathType"`
	// ActivePath 是客户端当前实际选中的路径，例如 direct_udp / relay_udp / relay_tcp。
	ActivePath string `json:"activePath,omitempty"`
	// Endpoint 是采样对应的端点。
	Endpoint string `json:"endpoint,omitempty"`
	// DerpNodeID 是采样对应的 relay/DERP 节点。
	DerpNodeID string `json:"derpNodeId,omitempty"`
	// ObservedRttMs 是观测 RTT。
	ObservedRttMs *uint32 `json:"observedRttMs,omitempty"`
	// PacketLossPpm 是观测丢包率。
	PacketLossPpm *uint32 `json:"packetLossPpm,omitempty"`
	// PathScore 是客户端本地评分。
	PathScore *uint32 `json:"pathScore,omitempty"`
	// SourceCountryCode 是客户端所在国家。
	SourceCountryCode string `json:"sourceCountryCode,omitempty"`
	// RelayCountryCode 是 relay 节点所在国家。
	RelayCountryCode string `json:"relayCountryCode,omitempty"`
	// PeerCountryCode 是对端所在国家。
	PeerCountryCode string `json:"peerCountryCode,omitempty"`
	// CrossCountry 标记这条质量样本是否跨国。
	CrossCountry *bool `json:"crossCountry,omitempty"`
	// RelayMtu 是客户端当前使用的 relay MTU。
	RelayMtu *uint32 `json:"relayMtu,omitempty"`
	// MaxFramePayload 是客户端当前使用的最大 frame payload。
	MaxFramePayload *uint32 `json:"maxFramePayload,omitempty"`
	// PathDowngrades 是客户端本地路径降级次数。
	PathDowngrades uint64 `json:"pathDowngrades,omitempty"`
	// PathUpgrades 是客户端本地路径升级次数。
	PathUpgrades uint64 `json:"pathUpgrades,omitempty"`
	// LastPathChange 是最近一次路径切换原因。
	LastPathChange string `json:"lastPathChange,omitempty"`
	// SampledAtMs 是采样时间戳，毫秒。
	SampledAtMs uint64 `json:"sampledAtMs,omitempty"`
}

// RelayDataPlanePolicy 是服务端下发给客户端的 relay 数据面策略。
type RelayDataPlanePolicy struct {
	PolicyID            string   `json:"policyId,omitempty"`
	Version             int      `json:"version"`
	Scope               string   `json:"scope,omitempty"`
	NetworkID           string   `json:"networkId,omitempty"`
	TargetDeviceIDs     []string `json:"targetDeviceIds,omitempty"`
	PathType            string   `json:"pathType,omitempty"`
	PreferredPathTypes  []string `json:"preferredPathTypes,omitempty"`
	RecommendationLevel *uint8   `json:"recommendationLevel,omitempty"`
	ExecutionLevel      *uint8   `json:"executionLevel,omitempty"`
	RelayMtu            uint32   `json:"relayMtu"`
	MaxFramePayload     uint32   `json:"maxFramePayload"`
	Reason              string   `json:"reason,omitempty"`
	TTLMS               uint64   `json:"ttlMs,omitempty"`
	EffectiveMS         uint64   `json:"effectiveMs,omitempty"`
	UpdatedAtMS         uint64   `json:"updatedAtMs,omitempty"`
}

// RelayPolicyReport 是客户端上报的 relay 数据面策略实际执行结果。
type RelayPolicyReport struct {
	NetworkID         string   `json:"networkId"`
	DeviceID          string   `json:"deviceId,omitempty"`
	PolicyID          string   `json:"policyId,omitempty"`
	Scope             string   `json:"scope,omitempty"`
	TargetDeviceIDs   []string `json:"targetDeviceIds,omitempty"`
	PathType          string   `json:"pathType,omitempty"`
	RelayMtu          uint32   `json:"relayMtu,omitempty"`
	MaxFramePayload   uint32   `json:"maxFramePayload,omitempty"`
	ExecutionLevel    *uint8   `json:"executionLevel,omitempty"`
	Applied           bool     `json:"applied"`
	Reason            string   `json:"reason,omitempty"`
	PolicyUpdatedAtMS uint64   `json:"policyUpdatedAtMs,omitempty"`
	ReportedAtMS      uint64   `json:"reportedAtMs,omitempty"`
}

// RelayNodeHeartbeat is published by server-relay through MQTT.
type RelayNodeHeartbeat struct {
	NodeID         string `json:"nodeId"`
	ClusterID      string `json:"clusterId,omitempty"`
	CountryCode    string `json:"countryCode,omitempty"`
	CityCode       string `json:"cityCode,omitempty"`
	Transport      string `json:"transport,omitempty"`
	Address        string `json:"address,omitempty"`
	Healthy        bool   `json:"healthy"`
	ActiveSessions int    `json:"activeSessions,omitempty"`
	ReportedAtMs   uint64 `json:"reportedAtMs,omitempty"`
}

// PathOption 描述一条可尝试的连接路径。
type PathOption struct {
	// PathType 是路径类型，例如 direct_udp / relay_udp / relay_tcp / relay_http3 / relay_tls。
	PathType string `json:"pathType"`
	// Endpoint 是目标端点。
	Endpoint string `json:"endpoint"`
	// Priority 是优先级，数值越小优先级越高。
	Priority int `json:"priority"`
}

// RelayTicket 描述控制面签发的 relay 回退票据。
type RelayTicket struct {
	// TicketID 是票据 ID。
	TicketID string `json:"ticketId"`
	// NetworkID 是所属网络 ID。
	NetworkID string `json:"networkId"`
	// SessionID 是 relay 会话 ID。
	SessionID string `json:"sessionId"`
	// SrcNodeID 是源节点 ID。
	SrcNodeID string `json:"srcNodeId"`
	// DstNodeID 是目标节点 ID。
	DstNodeID string `json:"dstNodeId"`
	// DerpClusterID 是票据允许使用的 DERP 集群。
	DerpClusterID string `json:"derpClusterId,omitempty"`
	// CountryCode 是选中的国家编码。
	CountryCode string `json:"countryCode,omitempty"`
	// CityCode 是选中的城市编码。
	CityCode string `json:"cityCode,omitempty"`
	// AllowedDerpNodeIDs 是允许的 DERP 节点白名单。
	AllowedDerpNodeIDs []string `json:"allowedDerpNodeIds,omitempty"`
	// RelayURL 是 relay 接入地址。
	RelayURL string `json:"relayUrl"`
	// ExpiresAt 是过期时间。
	ExpiresAt string `json:"expiresAt"`
	// SessionKey 是会话密钥或派生密钥。
	SessionKey string `json:"sessionKey,omitempty"`
	// Signature 是控制面签名。
	Signature string `json:"signature"`
}

// ConnectPlan 描述控制面给节点的连接计划。
type ConnectPlan struct {
	// PeerNodeID 是目标对等节点 ID。
	PeerNodeID string `json:"peerNodeId"`
	// PreferDirect 表示应优先尝试直连。
	PreferDirect bool `json:"preferDirect"`
	// Paths 是可尝试的连接路径列表。
	Paths []PathOption `json:"paths,omitempty"`
	// DerpClusterID 是建议使用的 DERP 集群。
	DerpClusterID string `json:"derpClusterId,omitempty"`
	// PreferredDerpNodeIDs 是建议优先的 DERP 节点列表。
	PreferredDerpNodeIDs []string `json:"preferredDerpNodeIds,omitempty"`
	// RelayTicket 是 relay 回退时附带的票据。
	RelayTicket *RelayTicket `json:"relayTicket,omitempty"`
}

// ConnectionState 用于向控制面回报本地连接进度。
type ConnectionState struct {
	// NetworkID 是当前会话所属的逻辑网络 ID。
	NetworkID string `json:"networkId"`
	// PeerNodeID 是当前上报状态的远端节点 ID。
	PeerNodeID string `json:"peerNodeId"`
	// Path 是当前连接使用的路径类型，例如 direct_udp / relay_udp / relay_tcp / relay_http3 / relay_tls。
	Path string `json:"path"`
	// State 是标准化状态值，例如 connecting、connected、failed 或 closed。
	State string `json:"state"`
	// Reason 在当前状态表示失败或回退时填写原因。
	Reason string `json:"reason,omitempty"`
	// ObservedRttMs 是客户端观测到的链路 RTT。
	ObservedRttMs *uint32 `json:"observedRttMs,omitempty"`
	// PacketLossPpm 是客户端观测到的丢包率，单位 ppm。
	PacketLossPpm *uint32 `json:"packetLossPpm,omitempty"`
	// PathScore 是客户端本地路径评分。
	PathScore *uint32 `json:"pathScore,omitempty"`
	// DerpNodeID 是当前连接使用的 DERP/relay 节点。
	DerpNodeID string `json:"derpNodeId,omitempty"`
}

// DisconnectNotice 用于通知连接断开。
type DisconnectNotice struct {
	// NetworkID 是所属网络。
	NetworkID string `json:"networkId"`
	// PeerNodeID 是对端节点。
	PeerNodeID string `json:"peerNodeId"`
	// Reason 是断开原因。
	Reason string `json:"reason,omitempty"`
}

// NetworkRestartRequired 用于通知客户端网络配置已变更，需要重载隧道。
type NetworkRestartRequired struct {
	// NetworkID 是发生配置变化的网络。
	NetworkID string `json:"networkId"`
	// Revision 是最新网络版本。
	Revision uint64 `json:"revision,omitempty"`
	// Reason 是重启原因说明。
	Reason string `json:"reason,omitempty"`
	// DefaultSubnetCIDR 是变更后的默认网段。
	DefaultSubnetCIDR string `json:"defaultSubnetCidr,omitempty"`
}

// DeviceIPReassigned 用于通知客户端某台设备在网络中的虚拟 IP 已被重绑。
type DeviceIPReassigned struct {
	// NetworkID 是发生 IP 调整的网络。
	NetworkID string `json:"networkId"`
	// DeviceID 是被调整的设备 ID。
	DeviceID string `json:"deviceId"`
	// AttachmentID 是对应的网络挂载关系。
	AttachmentID string `json:"attachmentId"`
	// VirtualIP 是最新生效的虚拟 IP。
	VirtualIP string `json:"virtualIp"`
	// Reason 是重绑原因说明。
	Reason string `json:"reason,omitempty"`
}

// DeviceNetworkDisabled 用于通知客户端当前设备的网络绑定已被停用。
type DeviceNetworkDisabled struct {
	// NetworkID 是被停用的网络。
	NetworkID string `json:"networkId"`
	// DeviceID 是被停用的设备 ID。
	DeviceID string `json:"deviceId"`
	// AttachmentID 是对应的网络挂载关系。
	AttachmentID string `json:"attachmentId,omitempty"`
	// Reason 是停用原因说明。
	Reason string `json:"reason,omitempty"`
}

// ActiveNetworkEnabled 用于通知客户端当前用户的活动网络已切换到指定网络。
type ActiveNetworkEnabled struct {
	// UserID 是被通知的目标用户。
	UserID string `json:"userId"`
	// NetworkID 是最新启用的活动网络。
	NetworkID string `json:"networkId"`
	// Reason 描述触发启用的原因。
	Reason string `json:"reason,omitempty"`
}

// ControlSyncEvent 是多实例之间通过 Redis 同步的控制通道事件。
type ControlSyncEvent struct {
	// InstanceID 是发布该事件的实例标识。
	InstanceID string `json:"instanceId"`
	// Type 是事件类型，例如 peer_update / peer_remove。
	Type string `json:"type"`
	// NetworkID 是事件所属网络。
	NetworkID string `json:"networkId"`
	// SourceNodeID 是触发事件的节点。
	SourceNodeID string `json:"sourceNodeId"`
	// Revision 是网络版本号。
	Revision uint64 `json:"revision,omitempty"`
	// Peer 是 peer_update 时携带的节点快照。
	Peer *Peer `json:"peer,omitempty"`
	// PeerNodeID 是 peer_remove 时携带的节点 ID。
	PeerNodeID string `json:"peerNodeId,omitempty"`
	// TargetNodeID 是 peer_candidate / connect_plan 的目标节点。
	TargetNodeID string `json:"targetNodeId,omitempty"`
	// TargetUserID 是 user-scoped 事件的目标用户。
	TargetUserID string `json:"targetUserId,omitempty"`
	// Candidate 是 peer_candidate 时携带的候选信息。
	Candidate *PeerCandidate `json:"candidate,omitempty"`
	// Plan 是 connect_plan 时携带的连接计划。
	Plan *ConnectPlan `json:"plan,omitempty"`
	// Restart 是 network_restart_required 时携带的重启提示。
	Restart *NetworkRestartRequired `json:"restart,omitempty"`
	// DeviceIP 是 device_ip_reassigned 时携带的虚拟 IP 变更信息。
	DeviceIP *DeviceIPReassigned `json:"deviceIp,omitempty"`
	// DeviceDisabled 是 device_network_disabled 时携带的停用信息。
	DeviceDisabled *DeviceNetworkDisabled `json:"deviceDisabled,omitempty"`
	// ActiveNetwork 是 active_network_enabled 时携带的活动网络信息。
	ActiveNetwork *ActiveNetworkEnabled `json:"activeNetwork,omitempty"`
}
