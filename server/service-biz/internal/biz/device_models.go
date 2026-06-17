package biz

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
	OwnerID         string `json:"ownerId"`
	OwnerEmail      string `json:"ownerEmail,omitempty"`
	Name            string `json:"name"`
	Platform        string `json:"platform"`
	OSName          string `json:"osName,omitempty"`
	OSVersion       string `json:"osVersion,omitempty"`
	Alias           string `json:"alias,omitempty"`
	PublicKey       string `json:"publicKey,omitempty"`
	GlobalIP        string `json:"globalIp"`
	PrefixLen       int    `json:"prefixLen,omitempty"`
	GlobalCIDR      string `json:"globalCidr,omitempty"`
	SubnetID        string `json:"subnetId,omitempty"`
	SubnetCIDR      string `json:"subnetCidr,omitempty"`
	SubnetPrefixLen int    `json:"subnetPrefixLen,omitempty"`
	// RelayAllowed 标识当前接收端是否可以为该 peer 创建中继会话。
	RelayAllowed bool             `json:"relayAllowed,omitempty"`
	GlobalName   string           `json:"globalName"`
	Status       string           `json:"status"`
	CreatedAt    int64            `json:"createdAt"`
	UpdatedAt    int64            `json:"updatedAt"`
	Endpoints    []DeviceEndpoint `json:"endpoints,omitempty"`
}

// DeviceGroup 是用户侧设备分组。一个设备可以同时归属多个分组。
type DeviceGroup struct {
	GroupID   string `json:"groupId"`
	UserID    string `json:"userId"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// DeviceGroupMember 是设备和分组的多对多归属关系。
type DeviceGroupMember struct {
	GroupID  string `json:"groupId"`
	DeviceID string `json:"deviceId"`
	AddedAt  int64  `json:"addedAt"`
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
