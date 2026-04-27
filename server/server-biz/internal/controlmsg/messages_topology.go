package controlmsg

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
	// NodeID 是节点 ID。
	NodeID string `json:"nodeId"`
	// DeviceID 是业务设备 ID。
	DeviceID string `json:"deviceId"`
	// PublicKey 是节点公钥。
	PublicKey string `json:"publicKey"`
	// Status 是节点控制面可达状态；虚拟网络在线状态以 device network state 为准。
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
	// Wildcards 是通配解析记录，格式如 *.xx.com=10.0.0.2。
	Wildcards []string `json:"wildcards,omitempty"`
}

type AccessPolicy struct {
	ProductCode        string `json:"productCode,omitempty"`
	MaxActiveDevices   int    `json:"maxActiveDevices,omitempty"`
	BandwidthLimitMbps int    `json:"bandwidthLimitMbps,omitempty"`
	DNSAvailable       bool   `json:"dnsAvailable,omitempty"`
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
	DNS    DNSConfig    `json:"dns"`
	Policy AccessPolicy `json:"policy,omitempty"`
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
