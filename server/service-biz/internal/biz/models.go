package biz

import "encoding/json"

// User 是控制台用户/客户账号的核心身份实体。
type User struct {
	// UserID 是服务端生成的用户唯一 ID。
	UserID string `json:"userId"`
	// Email 是登录账号和客户识别主键。
	Email string `json:"email"`
	// Name 是用户显示名称。
	Name string `json:"name,omitempty"`
	// PasswordHash 仅服务端保存，不返回给外部接口。
	PasswordHash string `json:"-"`
	// Status 表示账号状态，例如 active/disabled。
	Status string `json:"status"`
	// CreatedAt 是创建时间，Unix 秒。
	CreatedAt int64 `json:"createdAt"`
	// UpdatedAt 是最近更新时间，Unix 秒。
	UpdatedAt int64 `json:"updatedAt"`
}

// UserSession 是用户登录后的访问会话。
type UserSession struct {
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
	Token     string `json:"token"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
}

// LoginFailure 记录登录失败计数，用于限流和临时封禁。
type LoginFailure struct {
	Key           string `json:"key"`
	FailedCount   int    `json:"failedCount"`
	FirstFailedAt int64  `json:"firstFailedAt"`
	LastFailedAt  int64  `json:"lastFailedAt"`
	BlockedUntil  int64  `json:"blockedUntil,omitempty"`
}

// AuditEvent 是服务端业务动作的审计记录。
type AuditEvent struct {
	EventID      string            `json:"eventId"`
	ActorType    string            `json:"actorType"`
	ActorID      string            `json:"actorId,omitempty"`
	ActorEmail   string            `json:"actorEmail,omitempty"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resourceType,omitempty"`
	ResourceID   string            `json:"resourceId,omitempty"`
	Status       string            `json:"status"`
	RemoteIP     string            `json:"remoteIp,omitempty"`
	Details      map[string]string `json:"details,omitempty"`
	CreatedAt    int64             `json:"createdAt"`
}

// AuditEventFilter 是审计查询条件。
type AuditEventFilter struct {
	ActorType    string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Status       string
	Limit        int
}

// AuthResponse 是登录、注册、续期接口返回的认证信息。
type AuthResponse struct {
	User    User        `json:"user"`
	Session UserSession `json:"session"`
}

// DeviceUserLoginPayload 是客户端设备登录完成后，通过控制通道下发给设备的用户授权载荷。
type DeviceUserLoginPayload struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken *string `json:"refreshToken,omitempty"`
	UserID       string  `json:"userId"`
	UserLabel    string  `json:"userLabel"`
	DeviceID     *string `json:"deviceId,omitempty"`
	VirtualIP    *string `json:"virtualIp,omitempty"`
	ExpiresIn    uint64  `json:"expiresIn,omitempty"`
	Action       string  `json:"action,omitempty"`
}

// ConsoleLoginKey 是桌面/移动客户端发起控制台登录时的一次性登录凭据。
type ConsoleLoginKey struct {
	LoginKey   string `json:"loginKey"`
	UserID     string `json:"userId"`
	DeviceID   string `json:"deviceId,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	ConsumedAt int64  `json:"consumedAt,omitempty"`
	Status     string `json:"status"`
}

// MQTTControlDelivery 记录一条通过 MQTT 下发到设备的控制任务及其投递状态。
type MQTTControlDelivery struct {
	DeliveryID    string          `json:"deliveryId"`
	DeviceID      string          `json:"deviceId"`
	MessageType   string          `json:"messageType,omitempty"`
	TaskID        string          `json:"taskId,omitempty"`
	Action        string          `json:"action,omitempty"`
	Status        string          `json:"status"`
	AttemptCount  int             `json:"attemptCount,omitempty"`
	Error         string          `json:"error,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	ProcessedAtMs int64           `json:"processedAtMs,omitempty"`
	CreatedAt     int64           `json:"createdAt,omitempty"`
	ExpiresAt     int64           `json:"expiresAt,omitempty"`
	PublishedAt   int64           `json:"publishedAt,omitempty"`
	AckedAt       int64           `json:"ackedAt"`
	UpdatedAt     int64           `json:"updatedAt"`
}

// DeviceSession 是设备侧长期会话，包含设备 token、刷新 token 和活跃网络列表。
type DeviceSession struct {
	SessionID            string   `json:"sessionId"`
	DeviceID             string   `json:"deviceId"`
	UserID               string   `json:"userId,omitempty"`
	DeviceToken          string   `json:"deviceToken"`
	DeviceTokenExpiresAt int64    `json:"deviceTokenExpiresAt"`
	DeviceRefreshToken   string   `json:"deviceRefreshToken"`
	RegisteredAt         int64    `json:"registeredAt"`
	LastRenewedAt        int64    `json:"lastRenewedAt"`
	ActiveNetworkIDs     []string `json:"activeNetworkIds"`
	State                string   `json:"state"`
}

// DeviceBootstrapKey 是控制台生成的设备引导密钥，用于无登录或安装脚本绑定设备。
type DeviceBootstrapKey struct {
	KeyID           string `json:"id"`
	Key             string `json:"key,omitempty"`
	KeyHash         string `json:"-"`
	CreatedByUserID string `json:"createdByUserId"`
	NetworkID       string `json:"networkId"`
	DeviceAlias     string `json:"deviceAlias,omitempty"`
	ExpiresAt       int64  `json:"expiresAt"`
	UsedAt          int64  `json:"usedAt,omitempty"`
	UsedByDeviceID  string `json:"usedByDeviceId,omitempty"`
	RevokedAt       int64  `json:"revokedAt,omitempty"`
	Status          string `json:"status"`
	CreatedAt       int64  `json:"createdAt"`
}

// Device 是 SLAN 网络内可被用户管理和加入虚拟网络的客户端设备。
type Device struct {
	// DeviceID 是设备在服务端的稳定 ID。
	DeviceID string `json:"deviceId"`
	// OwnerID 是设备归属用户 ID。
	OwnerID         string           `json:"ownerId"`
	OwnerEmail      string           `json:"ownerEmail,omitempty"`
	Name            string           `json:"name"`
	Platform        string           `json:"platform"`
	OSName          string           `json:"osName,omitempty"`
	OSVersion       string           `json:"osVersion,omitempty"`
	Alias           string           `json:"alias,omitempty"`
	PublicKey       string           `json:"publicKey,omitempty"`
	GlobalIP        string           `json:"globalIp"`
	PrefixLen       int              `json:"prefixLen,omitempty"`
	GlobalCIDR      string           `json:"globalCidr,omitempty"`
	SubnetID        string           `json:"subnetId,omitempty"`
	SubnetCIDR      string           `json:"subnetCidr,omitempty"`
	SubnetPrefixLen int              `json:"subnetPrefixLen,omitempty"`
	GlobalName      string           `json:"globalName"`
	Status          string           `json:"status"`
	CreatedAt       int64            `json:"createdAt"`
	UpdatedAt       int64            `json:"updatedAt"`
	Endpoints       []DeviceEndpoint `json:"endpoints,omitempty"`
}

// DeviceEndpoint 是设备上报的网络端点候选地址。
type DeviceEndpoint struct {
	Type      string `json:"type"`
	Address   string `json:"address"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

// ClientDownload 是客户端安装包发布记录。
type ClientDownload struct {
	DownloadID   string `json:"downloadId"`
	Platform     string `json:"platform"`
	PlatformName string `json:"platformName"`
	Version      string `json:"version"`
	Arch         string `json:"arch,omitempty"`
	Channel      string `json:"channel"`
	FileName     string `json:"fileName"`
	FileSize     int64  `json:"fileSize"`
	SHA256       string `json:"sha256,omitempty"`
	DownloadURL  string `json:"downloadUrl"`
	ReleaseNotes string `json:"releaseNotes,omitempty"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// UserAlias 是一个用户给另一个用户邮箱配置的本地显示别名。
type UserAlias struct {
	OwnerUserID string `json:"ownerUserId"`
	Email       string `json:"email"`
	Alias       string `json:"alias"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// DeviceOwner 记录设备当前或历史归属关系。
type DeviceOwner struct {
	OwnerRecordID string `json:"ownerRecordId"`
	DeviceID      string `json:"deviceId"`
	UserID        string `json:"userId"`
	Status        string `json:"status"`
	BoundAt       int64  `json:"boundAt"`
	UnboundAt     int64  `json:"unboundAt,omitempty"`
}

// DeviceOwnerChangeLog 记录设备归属转移历史。
type DeviceOwnerChangeLog struct {
	LogID      string `json:"logId"`
	DeviceID   string `json:"deviceId"`
	FromUserID string `json:"fromUserId,omitempty"`
	ToUserID   string `json:"toUserId"`
	Reason     string `json:"reason"`
	ChangedAt  int64  `json:"changedAt"`
}

// GlobalIPAddress 是全局 10.0.0.0/8 地址池中的单个地址分配记录。
type GlobalIPAddress struct {
	AddressID  string `json:"addressId"`
	SubnetID   string `json:"subnetId"`
	IP         string `json:"ip"`
	CIDRBlock  string `json:"cidrBlock"`
	Offset     uint32 `json:"offset"`
	DeviceID   string `json:"deviceId,omitempty"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"createdAt"`
	AssignedAt int64  `json:"assignedAt,omitempty"`
	ReleasedAt int64  `json:"releasedAt,omitempty"`
}

// IPAMSubnet 是服务端从全局地址池切分出的地址段。
type IPAMSubnet struct {
	SubnetID          string `json:"subnetId"`
	CIDRBlock         string `json:"cidrBlock"`
	BaseIP            string `json:"baseIp"`
	PrefixLength      int    `json:"prefixLength"`
	StartOffset       uint32 `json:"startOffset"`
	EndOffset         uint32 `json:"endOffset"`
	GeneratedCapacity int    `json:"generatedCapacity"`
	Status            string `json:"status"`
	CreatedAt         int64  `json:"createdAt"`
}

// Network 是用户创建的虚拟网络。
type Network struct {
	NetworkID   string `json:"networkId"`
	OwnerUserID string `json:"ownerUserId"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	TemplateKey string `json:"templateKey,omitempty"`
	Status      string `json:"status"`
	Default     bool   `json:"default"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// DeviceInvite 是邀请其它设备加入当前用户资源的短期凭证。
type DeviceInvite struct {
	InviteID         string `json:"inviteId"`
	InviterUserID    string `json:"inviterUserId,omitempty"`
	InviteCode       string `json:"inviteCode"`
	Status           string `json:"status"`
	CreatedAt        int64  `json:"createdAt"`
	ExpiresAt        int64  `json:"expiresAt"`
	AcceptedDeviceID string `json:"acceptedDeviceId,omitempty"`
	AcceptedUserID   string `json:"acceptedUserId,omitempty"`
	AcceptedAt       int64  `json:"acceptedAt,omitempty"`
}

// DeviceAccessGrant 是设备通过邀请建立的访问授权关系。
type DeviceAccessGrant struct {
	GrantID    string `json:"grantId"`
	DeviceID   string `json:"deviceId"`
	UserID     string `json:"userId"`
	GrantedBy  string `json:"grantedBy,omitempty"`
	InviteCode string `json:"inviteCode,omitempty"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"createdAt"`
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
	DefaultPolicy   string `json:"defaultPolicy"`
	Status          string `json:"status"`
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

// DeviceRuntimeStatus 是设备最近一次运行状态上报的聚合结果。
type DeviceRuntimeStatus struct {
	DeviceID        string `json:"deviceId"`
	HeartbeatOnline bool   `json:"heartbeatOnline"`
	NetworkEnabled  bool   `json:"networkEnabled"`
	DeviceEnabled   bool   `json:"deviceEnabled"`
	RxBytesTotal    uint64 `json:"rxBytesTotal"`
	TxBytesTotal    uint64 `json:"txBytesTotal"`
	LastSeenAt      int64  `json:"lastSeenAt"`
	LastReportAt    int64  `json:"lastReportAt"`
}

// DeviceRuntimeReportResult 是设备运行状态上报后的变更结果。
type DeviceRuntimeReportResult struct {
	DeviceID              string   `json:"deviceId"`
	NetworkEnabled        bool     `json:"networkEnabled"`
	NetworkEnabledChanged bool     `json:"networkEnabledChanged"`
	NetworkIDs            []string `json:"networkIds"`
	ChangedAt             int64    `json:"changedAt"`
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
	NetworkName   string `json:"networkName,omitempty"`
	NetworkCode   string `json:"networkCode,omitempty"`
	ConfigVersion int64  `json:"configVersion,omitempty"`
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

// RelayCandidate 是 biz 下发给客户端的数据面候选转发节点。
type RelayCandidate struct {
	EndpointID  string `json:"endpointId"`
	Transport   string `json:"transport"`
	Address     string `json:"address"`
	CountryCode string `json:"countryCode,omitempty"`
	RegionID    string `json:"regionId,omitempty"`
	ClusterID   string `json:"clusterId,omitempty"`
}

// RelayTicket 是客户端连接 UDP relay 或 DERP 前必须携带的短期授权票据。
type RelayTicket struct {
	TicketID           string   `json:"ticketId"`
	NetworkID          string   `json:"networkId"`
	SessionID          string   `json:"sessionId"`
	SrcNodeID          string   `json:"srcNodeId"`
	DstNodeID          string   `json:"dstNodeId"`
	DERPClusterID      string   `json:"derpClusterId,omitempty"`
	CountryCode        string   `json:"countryCode,omitempty"`
	CityCode           string   `json:"cityCode,omitempty"`
	AllowedDERPNodeIDs []string `json:"allowedDerpNodeIds"`
	RelayURL           string   `json:"relayUrl"`
	ExpiresAt          string   `json:"expiresAt"`
	SessionKey         string   `json:"sessionKey"`
	Signature          string   `json:"signature"`
}
