package biz

type User struct {
	UserID       string `json:"userId"`
	Email        string `json:"email"`
	Name         string `json:"name,omitempty"`
	PasswordHash string `json:"-"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type UserSession struct {
	SessionID string `json:"sessionId"`
	UserID    string `json:"userId"`
	Token     string `json:"token"`
	CreatedAt int64  `json:"createdAt"`
	ExpiresAt int64  `json:"expiresAt"`
}

type AuthResponse struct {
	User    User        `json:"user"`
	Session UserSession `json:"session"`
}

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

type ConsoleLoginKey struct {
	LoginKey   string `json:"loginKey"`
	UserID     string `json:"userId"`
	DeviceID   string `json:"deviceId,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	ConsumedAt int64  `json:"consumedAt,omitempty"`
	Status     string `json:"status"`
}

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

type Device struct {
	DeviceID   string `json:"deviceId"`
	OwnerID    string `json:"ownerId"`
	OwnerEmail string `json:"ownerEmail,omitempty"`
	Name       string `json:"name"`
	Platform   string `json:"platform"`
	OSName     string `json:"osName,omitempty"`
	OSVersion  string `json:"osVersion,omitempty"`
	Alias      string `json:"alias,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
	GlobalIP   string `json:"globalIp"`
	GlobalName string `json:"globalName"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

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

type UserAlias struct {
	OwnerUserID string `json:"ownerUserId"`
	Email       string `json:"email"`
	Alias       string `json:"alias"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type DeviceOwner struct {
	OwnerRecordID string `json:"ownerRecordId"`
	DeviceID      string `json:"deviceId"`
	UserID        string `json:"userId"`
	Status        string `json:"status"`
	BoundAt       int64  `json:"boundAt"`
	UnboundAt     int64  `json:"unboundAt,omitempty"`
}

type DeviceOwnerChangeLog struct {
	LogID      string `json:"logId"`
	DeviceID   string `json:"deviceId"`
	FromUserID string `json:"fromUserId,omitempty"`
	ToUserID   string `json:"toUserId"`
	Reason     string `json:"reason"`
	ChangedAt  int64  `json:"changedAt"`
}

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

type DeviceAccessGrant struct {
	GrantID    string `json:"grantId"`
	DeviceID   string `json:"deviceId"`
	UserID     string `json:"userId"`
	GrantedBy  string `json:"grantedBy,omitempty"`
	InviteCode string `json:"inviteCode,omitempty"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"createdAt"`
}

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

type NetworkDNSZone struct {
	ZoneID       string `json:"zoneId"`
	NetworkID    string `json:"networkId"`
	ZoneName     string `json:"zoneName"`
	ExposeGlobal bool   `json:"exposeGlobal"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
}

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

type SecurityGroup struct {
	SecurityGroupID string `json:"securityGroupId"`
	NetworkID       string `json:"networkId"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	DefaultPolicy   string `json:"defaultPolicy"`
	Status          string `json:"status"`
	CreatedAt       int64  `json:"createdAt"`
}

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

type GlobalDNSRecord struct {
	Name     string `json:"name"`
	DeviceID string `json:"deviceId"`
	Value    string `json:"value"`
}

type NetworkConfig struct {
	NetworkID       string              `json:"networkId"`
	NetworkName     string              `json:"networkName,omitempty"`
	NetworkCode     string              `json:"networkCode,omitempty"`
	ConfigVersion   int64               `json:"configVersion,omitempty"`
	DeviceID        string              `json:"deviceId"`
	GlobalIP        string              `json:"globalIp"`
	GlobalName      string              `json:"globalName"`
	Peers           []Device            `json:"peers"`
	SecurityGroups  []SecurityGroup     `json:"securityGroups"`
	Rules           []SecurityGroupRule `json:"rules"`
	DNSZones        []NetworkDNSZone    `json:"dnsZones"`
	DNSRecords      []NetworkDNSRecord  `json:"dnsRecords"`
	RelayCandidates []RelayCandidate    `json:"relayCandidates,omitempty"`
}

type RelayCandidate struct {
	EndpointID  string `json:"endpointId"`
	Transport   string `json:"transport"`
	Address     string `json:"address"`
	CountryCode string `json:"countryCode,omitempty"`
	RegionID    string `json:"regionId,omitempty"`
	ClusterID   string `json:"clusterId,omitempty"`
}

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
