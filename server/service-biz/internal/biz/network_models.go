package biz

// Network 是用户创建的虚拟网络。
type Network struct {
	NetworkID        string `json:"networkId"`
	OwnerUserID      string `json:"ownerUserId"`
	Name             string `json:"name"`
	Code             string `json:"code"`
	TemplateKey      string `json:"templateKey,omitempty"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
	Default          bool   `json:"default"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

// NetworkDevice 是设备加入某个虚拟网络后的成员关系。
type NetworkDevice struct {
	NetworkDeviceID string `json:"networkDeviceId"`
	NetworkID       string `json:"networkId"`
	DeviceID        string `json:"deviceId"`
	OwnerUserID     string `json:"ownerUserId"`
	Alias           string `json:"alias,omitempty"`
	Enabled         bool   `json:"enabled"`
	Status          string `json:"status"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

// NetworkDNSZone 是虚拟网络内的 DNS Zone 配置。
type NetworkDNSZone struct {
	ZoneID       string `json:"zoneId"`
	NetworkID    string `json:"networkId"`
	ZoneName     string `json:"zoneName"`
	ExposeGlobal bool   `json:"exposeGlobal"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
}

// NetworkDNSRecord 是虚拟网络内的 DNS 解析记录。
type NetworkDNSRecord struct {
	RecordID       string `json:"recordId"`
	ZoneID         string `json:"zoneId"`
	NetworkID      string `json:"networkId"`
	Name           string `json:"name"`
	FQDN           string `json:"fqdn"`
	RecordType     string `json:"recordType"`
	TargetDeviceID string `json:"targetDeviceId,omitempty"`
	TargetIP       string `json:"targetIp,omitempty"`
	CNAME          string `json:"cname,omitempty"`
	Port           string `json:"port,omitempty"`
	TTL            int    `json:"ttl"`
	Status         string `json:"status"`
	CreatedAt      int64  `json:"createdAt"`
}

// PublicDomainMapping 描述公网域名到虚拟网络内设备服务的映射。
type PublicDomainMapping struct {
	MappingID    string `json:"mappingId"`
	NetworkID    string `json:"networkId"`
	Alias        string `json:"alias"`
	PublicDomain string `json:"publicDomain"`
	SourceRecord string `json:"sourceRecord"`
	DeviceID     string `json:"deviceId"`
	Protocol     string `json:"protocol"`
	Port         string `json:"port"`
	ExternalPort string `json:"externalPort"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// SecurityGroup 是虚拟网络内的访问控制策略集合。
type SecurityGroup struct {
	SecurityGroupID string `json:"securityGroupId"`
	NetworkID       string `json:"networkId"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Status          string `json:"-"`
	CreatedAt       int64  `json:"createdAt"`
}

// SecurityGroupRule 是安全组中的单条入站或出站规则。
type SecurityGroupRule struct {
	RuleID          string `json:"ruleId"`
	SecurityGroupID string `json:"securityGroupId"`
	Direction       string `json:"direction"`
	Priority        int    `json:"priority"`
	Action          string `json:"action"`
	Protocol        string `json:"protocol"`
	PortFrom        int    `json:"portFrom"`
	PortTo          int    `json:"portTo"`
	PeerType        string `json:"peerType"`
	PeerValue       string `json:"peerValue"`
	Description     string `json:"description,omitempty"`
	Enabled         bool   `json:"enabled"`
	CreatedAt       int64  `json:"createdAt"`
}

// NetworkConfigVersion 记录设备网络配置版本的生成和应用状态。
type NetworkConfigVersion struct {
	ConfigID      string `json:"configId"`
	NetworkID     string `json:"networkId"`
	DeviceID      string `json:"deviceId"`
	ConfigVersion int64  `json:"configVersion"`
	ConfigHash    string `json:"configHash"`
	PushedAt      int64  `json:"pushedAt"`
	AppliedAt     int64  `json:"appliedAt,omitempty"`
	Status        string `json:"status"`
}

// GlobalDNSRecord 是下发到客户端的全局 DNS 记录视图。
type GlobalDNSRecord struct {
	Name     string `json:"name"`
	DeviceID string `json:"deviceId"`
	Value    string `json:"value"`
}

// NetworkConfig 是下发给设备的数据面配置快照。
type NetworkConfig struct {
	// NetworkID 是配置所属虚拟网络。
	NetworkID string `json:"networkId"`
	// NetworkName 是网络显示名称。
	NetworkName      string `json:"networkName,omitempty"`
	NetworkCode      string `json:"networkCode,omitempty"`
	IntraGroupPolicy string `json:"intraGroupPolicy,omitempty"`
	NetworkCreatedAt int64  `json:"networkCreatedAt,omitempty"`
	ConfigVersion    int64  `json:"configVersion,omitempty"`
	// DeviceID 是接收该配置的本机设备 ID。
	DeviceID string `json:"deviceId"`
	// GlobalIP 是本机在 SLAN 网络内的虚拟 IP。
	GlobalIP        string              `json:"globalIp"`
	PrefixLen       int                 `json:"prefixLen,omitempty"`
	GlobalCIDR      string              `json:"globalCidr,omitempty"`
	SubnetID        string              `json:"subnetId,omitempty"`
	SubnetCIDR      string              `json:"subnetCidr,omitempty"`
	SubnetPrefixLen int                 `json:"subnetPrefixLen,omitempty"`
	GlobalName      string              `json:"globalName"`
	Peers           []Device            `json:"peers"`
	SecurityGroups  []SecurityGroup     `json:"securityGroups"`
	Rules           []SecurityGroupRule `json:"rules"`
	DNSZones        []NetworkDNSZone    `json:"dnsZones"`
	DNSRecords      []NetworkDNSRecord  `json:"dnsRecords"`
	RelayCandidates []RelayCandidate    `json:"relayCandidates,omitempty"`
}
