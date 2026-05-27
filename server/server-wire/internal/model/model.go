package model

import "time"

// PathKind 表示客户端数据面可选择的传输路径。
type PathKind string

const (
	// PathLANUDP 表示同局域网 UDP 直连。
	PathLANUDP PathKind = "lan_udp"
	// PathIPv6UDP 表示公网 IPv6 UDP 直连。
	PathIPv6UDP PathKind = "ipv6_udp"
	// PathDirectUDP 表示公网 IPv4 UDP 直连。
	PathDirectUDP PathKind = "direct_udp"
	// PathRelayUDP 表示通过 UDP relay 转发。
	PathRelayUDP PathKind = "relay_udp"
	// PathDerpTCP443 表示通过 DERP TCP 兜底转发；生产 443/TLS 由部署入口提供。
	PathDerpTCP443 PathKind = "derp_tcp_tls_443"
)

// PathProbe 是客户端上报的某条路径健康探测结果。
type PathProbe struct {
	Path             PathKind `json:"path"`
	Reachable        bool     `json:"reachable"`
	RTTMs            int      `json:"rttMs,omitempty"`
	LossPPM          int      `json:"lossPpm,omitempty"`
	JitterMs         int      `json:"jitterMs,omitempty"`
	ConsecutiveFails int      `json:"consecutiveFails,omitempty"`
	MTU              int      `json:"mtu,omitempty"`
	ObservedAt       int64    `json:"observedAt,omitempty"`
}

// RelayTicket 是客户端连接 UDP relay 所需的短期授权票据。
type RelayTicket struct {
	TicketID     string    `json:"ticketId,omitempty"`
	PeerID       string    `json:"peerId,omitempty"`
	SessionID    string    `json:"sessionId,omitempty"`
	Path         PathKind  `json:"path,omitempty"`
	RegionID     string    `json:"regionId,omitempty"`
	NodeID       string    `json:"nodeId,omitempty"`
	Host         string    `json:"host,omitempty"`
	UDPPort      int       `json:"udpPort,omitempty"`
	Present      bool      `json:"present"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	ExpiresInMs  int64     `json:"expiresInMs,omitempty"`
	RenewAfterMs int64     `json:"renewAfterMs,omitempty"`
	Signature    string    `json:"signature,omitempty"`
}

// RelayNode 是可调度的 UDP relay 节点描述。
type RelayNode struct {
	RegionID  string `json:"regionId"`
	NodeID    string `json:"nodeId"`
	Host      string `json:"host"`
	UDPPort   int    `json:"udpPort"`
	AdminPort int    `json:"adminPort,omitempty"`
	Enabled   bool   `json:"enabled"`
	Healthy   bool   `json:"healthy"`
	Stale     bool   `json:"stale"`
	Priority  int    `json:"priority"`
}

// DerpNode 是 DERP 区域内的一个 TCP 转发节点。
type DerpNode struct {
	RegionID string `json:"regionId"`
	NodeID   string `json:"nodeId"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

// DerpRegion 是 DERP 地域及其节点集合。
type DerpRegion struct {
	RegionID string     `json:"regionId"`
	Name     string     `json:"name"`
	Nodes    []DerpNode `json:"nodes"`
}

// DerpMap 是客户端选择 DERP 节点时使用的全局节点地图。
type DerpMap struct {
	PreferredRegionID string       `json:"preferredRegionId,omitempty"`
	Regions           []DerpRegion `json:"regions"`
}

// DerpTicket 是客户端连接 DERP 节点所需的短期授权票据。
type DerpTicket struct {
	TicketID     string    `json:"ticketId"`
	PeerID       string    `json:"peerId"`
	NetworkID    string    `json:"networkId,omitempty"`
	Path         PathKind  `json:"path"`
	RegionID     string    `json:"regionId"`
	NodeID       string    `json:"nodeId"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ExpiresInMs  int64     `json:"expiresInMs,omitempty"`
	RenewAfterMs int64     `json:"renewAfterMs,omitempty"`
	Signature    string    `json:"signature,omitempty"`
}

// PeerSnapshot 是路径规划器输入的 peer 当前能力和健康状态快照。
type PeerSnapshot struct {
	PeerID                  string             `json:"peerId"`
	ActivePath              PathKind           `json:"activePath,omitempty"`
	SupportsLANDirect       bool               `json:"supportsLanDirect"`
	SupportsIPv6Direct      bool               `json:"supportsIpv6Direct"`
	SupportsDirectUDP       bool               `json:"supportsDirectUdp"`
	SupportsRelayUDP        bool               `json:"supportsRelayUdp"`
	SupportsDerpTCPTLS443   bool               `json:"supportsDerpTcpTls443"`
	EndpointChanged         bool               `json:"endpointChanged"`
	PreferIPv6              bool               `json:"preferIpv6"`
	PreferLAN               bool               `json:"preferLan"`
	KeepaliveIntervalSecs   int                `json:"keepaliveIntervalSecs,omitempty"`
	Probes                  []PathProbe        `json:"probes"`
	RelayTicket             RelayTicket        `json:"relayTicket"`
	RecentPathDowngrades    int                `json:"recentPathDowngrades,omitempty"`
	RecentPathUpgrades      int                `json:"recentPathUpgrades,omitempty"`
	RequireMtuRefresh       bool               `json:"requireMtuRefresh"`
	AllowEndpointRoaming    bool               `json:"allowEndpointRoaming"`
	AllowFastReselection    bool               `json:"allowFastReselection"`
	AllowRelayTicketRenewal bool               `json:"allowRelayTicketRenewal"`
	DerpHealth              []DerpHealthSample `json:"derpHealth,omitempty"`
}

// Endpoint 是 peer 的网络端点候选地址。
type Endpoint struct {
	Kind       string `json:"kind"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Reachable  bool   `json:"reachable,omitempty"`
	ObservedAt int64  `json:"observedAt,omitempty"`
}

// PeerRegistration 是 peer 注册到 server-wire 时提交的能力声明。
type PeerRegistration struct {
	PeerID                  string     `json:"peerId"`
	NetworkID               string     `json:"networkId,omitempty"`
	NodeID                  string     `json:"nodeId,omitempty"`
	PublicKey               string     `json:"publicKey,omitempty"`
	VirtualIPs              []string   `json:"virtualIps,omitempty"`
	AllowedIPs              []string   `json:"allowedIps,omitempty"`
	SupportsLANDirect       bool       `json:"supportsLanDirect"`
	SupportsIPv6Direct      bool       `json:"supportsIpv6Direct"`
	SupportsDirectUDP       bool       `json:"supportsDirectUdp"`
	SupportsRelayUDP        bool       `json:"supportsRelayUdp"`
	SupportsDerpTCPTLS443   bool       `json:"supportsDerpTcpTls443"`
	PreferIPv6              bool       `json:"preferIpv6"`
	PreferLAN               bool       `json:"preferLan"`
	AllowEndpointRoaming    bool       `json:"allowEndpointRoaming"`
	AllowFastReselection    bool       `json:"allowFastReselection"`
	AllowRelayTicketRenewal bool       `json:"allowRelayTicketRenewal"`
	KeepaliveIntervalSecs   int        `json:"keepaliveIntervalSecs,omitempty"`
	Endpoints               []Endpoint `json:"endpoints,omitempty"`
}

// PeerRecord 是服务端保存的 peer 当前状态。
type PeerRecord struct {
	PeerRegistration
	ActivePath           PathKind           `json:"activePath,omitempty"`
	RecentPathDowngrades int                `json:"recentPathDowngrades,omitempty"`
	RecentPathUpgrades   int                `json:"recentPathUpgrades,omitempty"`
	RequireMtuRefresh    bool               `json:"requireMtuRefresh"`
	EndpointChanged      bool               `json:"endpointChanged"`
	Probes               []PathProbe        `json:"probes,omitempty"`
	DerpHealth           []DerpHealthSample `json:"derpHealth,omitempty"`
	RelayTicket          RelayTicket        `json:"relayTicket,omitempty"`
	UpdatedAt            int64              `json:"updatedAt"`
}

// RegisterPeerRequest 是注册或刷新 peer 基础能力的请求。
type RegisterPeerRequest struct {
	Peer PeerRegistration `json:"peer"`
}

// RegisterPeerResponse 是 peer 注册响应。
type RegisterPeerResponse struct {
	Peer PeerRecord `json:"peer"`
}

// UpdateEndpointsRequest 是 peer 上报端点候选地址的请求。
type UpdateEndpointsRequest struct {
	PeerID    string     `json:"peerId"`
	Endpoints []Endpoint `json:"endpoints"`
}

// ReportPathHealthRequest 是 peer 上报路径健康探测的请求。
type ReportPathHealthRequest struct {
	PeerID string      `json:"peerId"`
	Probes []PathProbe `json:"probes"`
}

// DerpHealthSample 是客户端对某个 DERP 节点的健康探测样本。
type DerpHealthSample struct {
	RegionID   string `json:"regionId"`
	NodeID     string `json:"nodeId"`
	Reachable  bool   `json:"reachable"`
	RTTMs      int    `json:"rttMs,omitempty"`
	ObservedAt int64  `json:"observedAt,omitempty"`
}

// ReportDerpHealthRequest 是 peer 批量上报 DERP 健康样本的请求。
type ReportDerpHealthRequest struct {
	PeerID  string             `json:"peerId"`
	Samples []DerpHealthSample `json:"samples"`
}

// UpdateActivePathRequest 是 peer 上报当前实际使用路径的请求。
type UpdateActivePathRequest struct {
	PeerID string   `json:"peerId"`
	Path   PathKind `json:"path"`
}

// IssueRelayTicketRequest 是签发 UDP relay 票据的请求。
type IssueRelayTicketRequest struct {
	PeerID       string `json:"peerId"`
	TTLSeconds   int64  `json:"ttlSeconds,omitempty"`
	RenewAfterMs int64  `json:"renewAfterMs,omitempty"`
}

// IssueRelayTicketResponse 是 UDP relay 票据签发响应。
type IssueRelayTicketResponse struct {
	Ticket RelayTicket `json:"ticket"`
}

// GetDerpMapResponse 是 DERP map 查询响应。
type GetDerpMapResponse struct {
	Map DerpMap `json:"map"`
}

// IssueDerpTicketRequest 是签发 DERP 票据的请求。
type IssueDerpTicketRequest struct {
	PeerID       string `json:"peerId"`
	RegionID     string `json:"regionId"`
	NodeID       string `json:"nodeId"`
	TTLSeconds   int64  `json:"ttlSeconds,omitempty"`
	RenewAfterMs int64  `json:"renewAfterMs,omitempty"`
}

// IssueDerpTicketResponse 是 DERP 票据签发响应。
type IssueDerpTicketResponse struct {
	Ticket DerpTicket `json:"ticket"`
}

// PeerAuthzView 是 biz 返回给 wire 的 peer 授权视图。
type PeerAuthzView struct {
	PeerID      string   `json:"peerId"`
	NetworkID   string   `json:"networkId,omitempty"`
	NodeID      string   `json:"nodeId,omitempty"`
	Enabled     bool     `json:"enabled"`
	VirtualIPs  []string `json:"virtualIps,omitempty"`
	AllowedIPs  []string `json:"allowedIps,omitempty"`
	QuotaPolicy string   `json:"quotaPolicy,omitempty"`
}

// PeerRuntimeConfigView 是 wire 向 biz 查询到的 peer 运行配置视图。
type PeerRuntimeConfigView struct {
	PeerID                string     `json:"peerId"`
	NetworkID             string     `json:"networkId,omitempty"`
	NodeID                string     `json:"nodeId,omitempty"`
	VirtualIPs            []string   `json:"virtualIps,omitempty"`
	AllowedIPs            []string   `json:"allowedIps,omitempty"`
	KeepaliveIntervalSecs int        `json:"keepaliveIntervalSecs,omitempty"`
	NetworkEnabled        bool       `json:"networkEnabled"`
	PreferredPath         PathKind   `json:"preferredPath,omitempty"`
	Endpoints             []Endpoint `json:"endpoints,omitempty"`
}

// NetworkTopologyView 是某个虚拟网络内所有 peer 的拓扑视图。
type NetworkTopologyView struct {
	NetworkID string       `json:"networkId"`
	Peers     []PeerRecord `json:"peers"`
}

// PathPlanRequest 是请求生成路径规划的输入。
type PathPlanRequest struct {
	PeerID string       `json:"peerId,omitempty"`
	Peer   PeerSnapshot `json:"peer"`
}

// ScoredPath 是路径规划器输出的单条候选路径评分。
type ScoredPath struct {
	Path    PathKind `json:"path"`
	Score   int      `json:"score"`
	Reason  string   `json:"reason"`
	MTU     int      `json:"mtu,omitempty"`
	Primary bool     `json:"primary,omitempty"`
}

// KeepalivePlan 是客户端 keepalive 策略。
type KeepalivePlan struct {
	IntervalSecs int    `json:"intervalSecs"`
	Mode         string `json:"mode"`
}

// MtuPlan 是 MTU 探测策略。
type MtuPlan struct {
	ProbeRequired bool `json:"probeRequired"`
	TargetMTU     int  `json:"targetMtu,omitempty"`
}

// RelayTicketPlan 是 relay 票据续期策略。
type RelayTicketPlan struct {
	RenewRequired bool  `json:"renewRequired"`
	RenewWindowMs int64 `json:"renewWindowMs,omitempty"`
}

// RoamingPlan 是端点漂移后的漫游处理策略。
type RoamingPlan struct {
	Apply bool   `json:"apply"`
	Mode  string `json:"mode,omitempty"`
}

// PathPlan 是 server-wire 返回给客户端的数据面路径选择结果。
type PathPlan struct {
	// PreferredPath 是当前建议优先使用的路径。
	PreferredPath PathKind `json:"preferredPath"`
	// DegradedReason 描述 relay/DERP 等控制面不可用导致的降级原因。
	DegradedReason     string          `json:"degradedReason,omitempty"`
	FallbackOrder      []PathKind      `json:"fallbackOrder"`
	ScoredPaths        []ScoredPath    `json:"scoredPaths"`
	Keepalive          KeepalivePlan   `json:"keepalive"`
	MTU                MtuPlan         `json:"mtu"`
	Roaming            RoamingPlan     `json:"roaming"`
	RelayTicket        RelayTicketPlan `json:"relayTicket"`
	RelayCandidates    []RelayNode     `json:"relayCandidates,omitempty"`
	DerpCandidates     []DerpNode      `json:"derpCandidates,omitempty"`
	DerpTicket         *DerpTicket     `json:"derpTicket,omitempty"`
	FastReselection    bool            `json:"fastReselection"`
	IPv6Preferred      bool            `json:"ipv6Preferred"`
	LANDirectPreferred bool            `json:"lanDirectPreferred"`
}

// TicketKeyStatus 描述 server-wire 票据签名密钥配置和轮转状态。
type TicketKeyStatus struct {
	Source             string `json:"source"`
	KeyRingID          string `json:"keyRingId"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback"`
}
