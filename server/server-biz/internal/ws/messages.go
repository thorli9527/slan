package ws

// Envelope 是控制通道使用的通用 WebSocket 消息封装。
type Envelope struct {
	// Type 标识负载类型，例如 node_hello、network_map_response 或 connect_plan。
	Type string `json:"type"`
	// RequestID 在需要时用于关联请求和响应。
	RequestID string `json:"requestId,omitempty"`
	// Payload 承载具体消息体。
	Payload interface{} `json:"payload,omitempty"`
}

// NodeHello 是节点在控制通道上的第一条握手消息。
type NodeHello struct {
	// UserID 是所属用户 ID。
	UserID string `json:"userId"`
	// DeviceID 是业务设备 ID。
	DeviceID string `json:"deviceId"`
	// NodeID 是节点 ID。
	NodeID string `json:"nodeId"`
	// NetworkID 是目标网络 ID。
	NetworkID string `json:"networkId"`
	// SessionToken 是控制通道鉴权令牌。
	SessionToken string `json:"sessionToken"`
	// NodePublicKey 是节点公钥。
	NodePublicKey string `json:"nodePublicKey"`
	// Capabilities 是节点声明的能力集合。
	Capabilities []string `json:"capabilities,omitempty"`
}

// NodeHelloAck 用于确认节点握手。
type NodeHelloAck struct {
	// ControlSessionID 是控制通道会话 ID。
	ControlSessionID string `json:"controlSessionId"`
	// HeartbeatSeconds 是建议心跳间隔。
	HeartbeatSeconds int `json:"heartbeatSeconds"`
	// NetworkRevision 是当前网络地图版本号。
	NetworkRevision uint64 `json:"networkRevision"`
}

// Ping 是保活探测消息。
type Ping struct {
	// Timestamp 是发送时间戳。
	Timestamp int64 `json:"timestamp"`
}

// Pong 是保活应答消息。
type Pong struct {
	// Timestamp 是回显时间戳。
	Timestamp int64 `json:"timestamp"`
}

// NetworkMapRequest 用于请求完整网络地图。
type NetworkMapRequest struct {
	// NetworkID 是目标网络 ID。
	NetworkID string `json:"networkId"`
	// LastRevision 是客户端本地已知的地图版本。
	LastRevision uint64 `json:"lastRevision"`
}

// Endpoint 描述节点可用的一个候选地址。
type Endpoint struct {
	// Type 是端点类型，例如 lan / wan / reflexive / relay。
	Type string `json:"type"`
	// Address 是 ip:port 地址。
	Address string `json:"address"`
	// UpdatedAt 是最近更新时间戳。
	UpdatedAt int64 `json:"updatedAt"`
}

// Peer 描述网络地图中的一个对等节点。
type Peer struct {
	// NodeID 是节点 ID。
	NodeID string `json:"nodeId"`
	// DeviceID 是业务设备 ID。
	DeviceID string `json:"deviceId"`
	// PublicKey 是节点公钥。
	PublicKey string `json:"publicKey"`
	// Status 是节点在线状态。
	Status string `json:"status"`
	// RelayAllowed 表示该节点是否允许 relay 回退。
	RelayAllowed bool `json:"relayAllowed"`
	// VirtualIPs 是分配给该节点的虚拟 IP 列表。
	VirtualIPs []string `json:"virtualIps,omitempty"`
	// Endpoints 是节点候选端点。
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	// AllowedRoutes 是节点可达或允许的路由。
	AllowedRoutes []string `json:"allowedRoutes,omitempty"`
}

// Route 描述一条网络内路由。
type Route struct {
	// CIDR 是目标网段。
	CIDR string `json:"cidr"`
	// ViaNodeID 是下一跳节点。
	ViaNodeID string `json:"viaNodeId"`
	// Metric 是可选度量。
	Metric string `json:"metric,omitempty"`
}

// DNSConfig 描述网络级 DNS 信息。
type DNSConfig struct {
	// Servers 是 DNS 服务器列表。
	Servers []string `json:"servers,omitempty"`
	// SearchDomains 是搜索域列表。
	SearchDomains []string `json:"searchDomains,omitempty"`
}

// RelayEndpoint 描述某个 relay 区域下的具体入口点。
type RelayEndpoint struct {
	// EndpointID 是端点 ID。
	EndpointID string `json:"endpointId"`
	// Transport 是传输方式，例如 udp / tcp / quic。
	Transport string `json:"transport"`
	// Address 是接入地址。
	Address string `json:"address"`
}

// RelayRegion 描述一个 relay 区域。
type RelayRegion struct {
	// RegionID 是区域 ID。
	RegionID string `json:"regionId"`
	// RegionName 是区域名称。
	RegionName string `json:"regionName"`
	// Endpoints 是该区域可用 relay 端点列表。
	Endpoints []RelayEndpoint `json:"endpoints,omitempty"`
}

// DerpNode 描述一个 DERP 节点。
type DerpNode struct {
	// NodeID 是 DERP 节点 ID。
	NodeID string `json:"nodeId"`
	// Host 是主机名或 IP。
	Host string `json:"host"`
	// Port 是节点端口。
	Port int `json:"port"`
	// Transport 是传输类型。
	Transport string `json:"transport"`
	// Priority 是建议优先级。
	Priority int `json:"priority"`
	// Tags 是可选标签。
	Tags []string `json:"tags,omitempty"`
}

// DerpCluster 描述一个 DERP 集群。
type DerpCluster struct {
	// ClusterID 是集群 ID。
	ClusterID string `json:"clusterId"`
	// RegionID 是区域 ID。
	RegionID string `json:"regionId"`
	// RegionName 是区域名称。
	RegionName string `json:"regionName"`
	// RecommendedFanout 是建议热连接数。
	RecommendedFanout int `json:"recommendedFanout"`
	// Nodes 是集群节点列表。
	Nodes []DerpNode `json:"nodes,omitempty"`
}

// DerpMap 描述可用 DERP 集群视图。
type DerpMap struct {
	// ProbeIntervalSeconds 是建议探测周期。
	ProbeIntervalSeconds int `json:"probeIntervalSeconds"`
	// Clusters 是可用集群。
	Clusters []DerpCluster `json:"clusters,omitempty"`
}

// NetworkMap 是控制面下发给节点的网络视图。
type NetworkMap struct {
	// SelfUserID 是当前节点所属用户 ID。
	SelfUserID string `json:"selfUserId"`
	// SelfDeviceID 是当前节点关联设备 ID。
	SelfDeviceID string `json:"selfDeviceId"`
	// SelfNodeID 是当前节点 ID。
	SelfNodeID string `json:"selfNodeId"`
	// NetworkID 是所属网络 ID。
	NetworkID string `json:"networkId"`
	// Revision 是网络地图版本号。
	Revision uint64 `json:"revision"`
	// HeartbeatSeconds 是控制通道心跳间隔。
	HeartbeatSeconds int `json:"heartbeatSeconds"`
	// STUNServers 是 NAT 探测使用的 STUN 列表。
	STUNServers []string `json:"stunServers,omitempty"`
	// Peers 是当前网络中的对等节点列表。
	Peers []Peer `json:"peers,omitempty"`
	// Routes 是网络路由列表。
	Routes []Route `json:"routes,omitempty"`
	// RelayRegions 是可用 relay 区域列表。
	RelayRegions []RelayRegion `json:"relayRegions,omitempty"`
	// DNS 是网络级 DNS 配置。
	DNS DNSConfig `json:"dns"`
	// MTU 是建议 MTU。
	MTU int `json:"mtu,omitempty"`
}

// NetworkMapResponse 用于返回完整网络地图。
type NetworkMapResponse struct {
	// Map 是当前网络地图快照。
	Map NetworkMap `json:"map"`
}

// PeerUpdate 用于增量更新某个对等节点。
type PeerUpdate struct {
	// NetworkID 是所属网络。
	NetworkID string `json:"networkId"`
	// Revision 是更新后的地图版本号。
	Revision uint64 `json:"revision"`
	// Peer 是最新的对等节点快照。
	Peer Peer `json:"peer"`
}

// PeerRemove 用于通知对等节点被移除。
type PeerRemove struct {
	// NetworkID 是所属网络。
	NetworkID string `json:"networkId"`
	// Revision 是更新后的地图版本号。
	Revision uint64 `json:"revision"`
	// PeerNodeID 是被移除的对等节点 ID。
	PeerNodeID string `json:"peerNodeId"`
}

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
	// Endpoint 是用于直连尝试的网络地址。
	Endpoint string `json:"endpoint"`
	// Priority 是候选优先级，数值越小优先级越高。
	Priority int `json:"priority"`
}

// PathOption 描述一条可尝试的连接路径。
type PathOption struct {
	// PathType 是路径类型，例如 direct_udp / direct_ipv6 / relay。
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
	// Path 是当前连接使用的路径类型，例如 p2p / relay。
	Path string `json:"path"`
	// State 是标准化状态值，例如 connecting、connected、failed 或 closed。
	State string `json:"state"`
	// Reason 在当前状态表示失败或回退时填写原因。
	Reason string `json:"reason,omitempty"`
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

// ErrorMessage 是控制通道错误消息。
type ErrorMessage struct {
	// Code 是稳定错误码。
	Code string `json:"code"`
	// Message 是人类可读错误信息。
	Message string `json:"message"`
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
	// Candidate 是 peer_candidate 时携带的候选信息。
	Candidate *PeerCandidate `json:"candidate,omitempty"`
	// Plan 是 connect_plan 时携带的连接计划。
	Plan *ConnectPlan `json:"plan,omitempty"`
}
