package dto

// CreateControlSessionRequest 用于创建控制通道会话。
type CreateControlSessionRequest struct {
	// NodeID 是发起控制会话的节点 ID。
	NodeID string `json:"nodeId"`
	// NetworkID 是目标网络 ID。
	NetworkID string `json:"networkId"`
}

// BootstrapRequest 用于为指定节点会话加载运行时配置。
type BootstrapRequest struct {
	// NodeID 是即将启动运行时会话的节点 ID。
	NodeID string `json:"nodeId"`
	// NetworkID 是目标网络 ID。
	NetworkID string `json:"networkId"`
}

// DeviceBootstrap 返回设备本身及其当前全部子网挂载关系。
type DeviceBootstrap struct {
	// Device 是标准设备实体。
	Device Device `json:"device"`
	// Attachments 包含该设备所有子网落点和对应分配 IP。
	Attachments []SubnetAttachment `json:"attachments"`
}

// ControlPlaneConfig 包含控制通道所需的端点配置。
type ControlPlaneConfig struct {
	// ControlURL is the MQTT broker URL exposed with the public wsUrl JSON field.
	ControlURL string `json:"wsUrl"`
	// HeartbeatSeconds 是服务端期望的保活心跳间隔。
	HeartbeatSeconds int `json:"heartbeatSeconds"`
}

// RelayConfig 包含中继回退所需的端点配置。
type RelayConfig struct {
	// DefaultClusterID 是默认 relay 集群。
	DefaultClusterID string `json:"defaultClusterId"`
	// Countries 是 relay 四层拓扑。
	Countries []RelayCountry `json:"countries,omitempty"`
}

// RelayCountry 描述国家层级的 relay 拓扑。
type RelayCountry struct {
	// CountryCode 是国家编码，例如 CN。
	CountryCode string `json:"countryCode"`
	// CountryName 是国家名称。
	CountryName string `json:"countryName"`
	// Cities 是国家下的城市列表。
	Cities []RelayCity `json:"cities,omitempty"`
}

// RelayCity 描述城市层级的 relay 拓扑。
type RelayCity struct {
	// CityCode 是城市编码，例如 bj。
	CityCode string `json:"cityCode"`
	// CityName 是城市名称。
	CityName string `json:"cityName"`
	// Clusters 是城市下的集群列表。
	Clusters []RelayCluster `json:"clusters,omitempty"`
}

// RelayCluster 描述城市下的一个 relay 集群。
type RelayCluster struct {
	// ClusterID 是集群 ID。
	ClusterID string `json:"clusterId"`
	// ClusterName 是集群名称。
	ClusterName string `json:"clusterName"`
	// Nodes 是集群内节点列表。
	Nodes []RelayNode `json:"nodes,omitempty"`
}

// RelayNode 描述一个 relay 节点。
type RelayNode struct {
	// NodeID 是节点 ID。
	NodeID string `json:"nodeId"`
	// Transport 是传输类型，例如 udp / tcp / quic。
	Transport string `json:"transport"`
	// Address 是公网接入地址。
	Address string `json:"address"`
	// Priority 是优先级。
	Priority int `json:"priority"`
	// Tags 是可选标签。
	Tags []string `json:"tags,omitempty"`
}

// DerpNode 描述一个 DERP 节点。
type DerpNode struct {
	// NodeID 是 DERP 节点 ID。
	NodeID string `json:"nodeId"`
	// Host 是节点主机名或 IP。
	Host string `json:"host"`
	// Port 是节点监听端口。
	Port int `json:"port"`
	// Transport 是传输类型，例如 udp / tcp / quic。
	Transport string `json:"transport"`
	// Priority 是控制面建议优先级。
	Priority int `json:"priority"`
	// Tags 是可选标签。
	Tags []string `json:"tags,omitempty"`
}

// DerpCluster 描述一个 DERP 集群。
type DerpCluster struct {
	// ClusterID 是集群 ID。
	ClusterID string `json:"clusterId"`
	// ClusterName 是集群名称。
	ClusterName string `json:"clusterName,omitempty"`
	// RegionID 是区域 ID。
	RegionID string `json:"regionId"`
	// RegionName 是区域名称。
	RegionName string `json:"regionName"`
	// CountryCode 是国家编码。
	CountryCode string `json:"countryCode,omitempty"`
	// CountryName 是国家名称。
	CountryName string `json:"countryName,omitempty"`
	// CityCode 是城市编码。
	CityCode string `json:"cityCode,omitempty"`
	// CityName 是城市名称。
	CityName string `json:"cityName,omitempty"`
	// RecommendedFanout 是建议客户端建立的热连接数量。
	RecommendedFanout int `json:"recommendedFanout"`
	// Nodes 是集群内 DERP 节点列表。
	Nodes []DerpNode `json:"nodes,omitempty"`
}

// DerpMap 是控制面下发给客户端的 DERP 集群视图。
type DerpMap struct {
	// ProbeIntervalSeconds 是客户端建议探测周期。
	ProbeIntervalSeconds int `json:"probeIntervalSeconds"`
	// Clusters 是可用 DERP 集群列表。
	Clusters []DerpCluster `json:"clusters,omitempty"`
}

// Endpoint 描述节点当前可用的一个候选网络端点。
type Endpoint struct {
	// Type 标识端点类型，例如 lan / wan / reflexive / relay。
	Type string `json:"type"`
	// Address 是 ip:port 形式的地址。
	Address string `json:"address"`
	// UpdatedAt 是最近更新时间，使用 unix 秒时间戳。
	UpdatedAt int64 `json:"updatedAt"`
}

// Peer 描述网络地图中的一个对等节点。
type Peer struct {
	// NodeID 是对等节点 ID。
	NodeID string `json:"nodeId"`
	// DeviceID 是关联设备 ID。
	DeviceID string `json:"deviceId"`
	// PublicKey 是对等节点公钥。
	PublicKey string `json:"publicKey"`
	// Status 是节点控制面可达状态；虚拟网络在线状态以 device network state 为准。
	Status string `json:"status"`
	// RelayAllowed 表示是否允许走 relay 回退。
	RelayAllowed bool `json:"relayAllowed"`
	// VirtualIPs 是分配给该节点的虚拟 IP 列表。
	VirtualIPs []string `json:"virtualIps,omitempty"`
	// Endpoints 是当前控制面已知的候选端点列表。
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	// AllowedRoutes 是该节点允许通告或可达的路由列表。
	AllowedRoutes []string `json:"allowedRoutes,omitempty"`
}

// Route 描述一条网络内可见的逻辑路由。
type Route struct {
	// CIDR 是目标网段。
	CIDR string `json:"cidr"`
	// ViaNodeID 是下一跳节点 ID。
	ViaNodeID string `json:"viaNodeId"`
	// Metric 是可选的度量值。
	Metric string `json:"metric,omitempty"`
}

// DNSConfig 描述网络级 DNS 配置。
type DNSConfig struct {
	// Servers 是 DNS 服务器地址列表。
	Servers []string `json:"servers,omitempty"`
	// SearchDomains 是搜索域列表。
	SearchDomains []string `json:"searchDomains,omitempty"`
	// Wildcards 是通配解析记录，格式如 *.xx.com=100.64.0.10。
	Wildcards []string `json:"wildcards,omitempty"`
}

type AccessPolicy struct {
	PlanCode                string `json:"planCode,omitempty"`
	MaxActiveDevices        int    `json:"maxActiveDevices,omitempty"`
	BandwidthLimitMbps      int    `json:"bandwidthLimitMbps,omitempty"`
	RelayBandwidthLimitKbps int    `json:"relayBandwidthLimitKbps,omitempty"`
	RelayIngressKbps        int    `json:"relayIngressKbps,omitempty"`
	RelayEgressKbps         int    `json:"relayEgressKbps,omitempty"`
	UDPIngressKbps          int    `json:"udpIngressKbps,omitempty"`
	UDPEgressKbps           int    `json:"udpEgressKbps,omitempty"`
	P2PUnlimited            bool   `json:"p2pUnlimited,omitempty"`
	DNSAvailable            bool   `json:"dnsAvailable,omitempty"`
}

// RelayEndpoint 描述一个 relay 区域下的具体接入点。
type RelayEndpoint struct {
	// EndpointID 是 relay 端点 ID。
	EndpointID string `json:"endpointId"`
	// Transport 是传输类型，例如 udp / tcp / quic。
	Transport string `json:"transport"`
	// Address 是 relay 监听地址。
	Address string `json:"address"`
}

// RelayRegion 描述一个 relay 区域。
type RelayRegion struct {
	// RegionID 是区域 ID。
	RegionID string `json:"regionId"`
	// RegionName 是区域名称。
	RegionName string `json:"regionName"`
	// CountryCode 是国家编码。
	CountryCode string `json:"countryCode,omitempty"`
	// CountryName 是国家名称。
	CountryName string `json:"countryName,omitempty"`
	// CityCode 是城市编码。
	CityCode string `json:"cityCode,omitempty"`
	// CityName 是城市名称。
	CityName string `json:"cityName,omitempty"`
	// ClusterID 是集群 ID。
	ClusterID string `json:"clusterId,omitempty"`
	// ClusterName 是集群名称。
	ClusterName string `json:"clusterName,omitempty"`
	// Endpoints 是该区域下的 relay 接入点。
	Endpoints []RelayEndpoint `json:"endpoints,omitempty"`
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
	// DNS 是可选 DNS 配置。
	DNS    DNSConfig    `json:"dns"`
	Policy AccessPolicy `json:"policy,omitempty"`
	// MTU 是建议 MTU。
	MTU int `json:"mtu,omitempty"`
}

// BootstrapResponse 是客户端核心启动时使用的完整启动载荷。
type BootstrapResponse struct {
	// ControlSessionID 是控制通道会话 ID。
	ControlSessionID string `json:"controlSessionId,omitempty"`
	// SessionToken 是控制通道鉴权令牌。
	SessionToken string `json:"sessionToken,omitempty"`
	// Device 是设备启动视图，包含当前子网挂载信息。
	Device DeviceBootstrap `json:"device"`
	// Networks 是该设备可见的网络拓扑。
	Networks []NetworkDetail `json:"networks"`
	// ControlPlane 是运行时控制通道配置。
	ControlPlane ControlPlaneConfig `json:"controlPlane"`
	// STUNServers 是用于 NAT 探测的候选 STUN 服务列表。
	STUNServers []string `json:"stunServers"`
	// Relay 是中继回退配置。
	Relay RelayConfig `json:"relay"`
	// DerpMap 是 DERP 集群视图。
	DerpMap DerpMap `json:"derpMap"`
	// NetworkMap 是初始网络地图。
	NetworkMap NetworkMap `json:"networkMap"`
}

// ControlSessionResponse 描述控制通道会话的创建结果。
type ControlSessionResponse struct {
	// ControlSessionID 是控制面分配的会话 ID。
	ControlSessionID string `json:"controlSessionId"`
	// SessionToken 是控制通道鉴权令牌。
	SessionToken string `json:"sessionToken"`
	// ControlPlane 是控制面 MQTT 配置。
	ControlPlane ControlPlaneConfig `json:"controlPlane"`
	// NetworkMap 是当前会话的初始网络地图。
	NetworkMap NetworkMap `json:"networkMap"`
}

// RelayTicketRequest 用于向控制面申请中继回退授权。
type RelayTicketRequest struct {
	// NetworkID 是本次请求中继会话所属的逻辑网络 ID。
	NetworkID string `json:"networkId"`
	// SrcNodeID 是本地源节点 ID。
	SrcNodeID string `json:"srcNodeId"`
	// DstNodeID 是目标对等节点 ID。
	DstNodeID string `json:"dstNodeId"`
	// DerpClusterID 是期望使用的 DERP 集群 ID。
	DerpClusterID string `json:"derpClusterId,omitempty"`
	// PreferredDerpNodeIDs 是客户端偏好的 DERP 节点列表。
	PreferredDerpNodeIDs []string `json:"preferredDerpNodeIds,omitempty"`
	// Reason 描述无法建立直连的原因。
	Reason string `json:"reason"`
	// RelayRegionID 是期望使用的 relay 区域 ID。
	RelayRegionID string `json:"relayRegionId,omitempty"`
}

// RelayTicket 是提供给中继数据面消费的授权票据。
type RelayTicket struct {
	// TicketID 是唯一的中继授权标识。
	TicketID string `json:"ticketId"`
	// NetworkID 是 relay 会话所属网络 ID。
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
	// AllowedDerpNodeIDs 是票据允许的 DERP 节点白名单。
	AllowedDerpNodeIDs []string `json:"allowedDerpNodeIds,omitempty"`
	// RelayURL 是应当接收该票据的中继端点。
	RelayURL string `json:"relayUrl"`
	// ExpiresAt 是票据过期时间。
	ExpiresAt string `json:"expiresAt"`
	// SessionKey 是可选的派生密钥或不透明会话密钥。
	SessionKey string `json:"sessionKey,omitempty"`
	// Signature 是供中继校验的控制面签名。
	Signature string `json:"signature"`
}
