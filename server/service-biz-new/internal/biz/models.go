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

type Device struct {
	DeviceID   string `json:"deviceId"`
	OwnerID    string `json:"ownerId"`
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

type Workspace struct {
	WorkspaceID string `json:"workspaceId"`
	OwnerUserID string `json:"ownerUserId"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	TemplateKey string `json:"templateKey,omitempty"`
	Status      string `json:"status"`
	Default     bool   `json:"default"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type WorkspaceMember struct {
	MemberID    string `json:"memberId"`
	WorkspaceID string `json:"workspaceId"`
	UserID      string `json:"userId"`
	Email       string `json:"email,omitempty"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	JoinedAt    int64  `json:"joinedAt"`
}

type WorkspaceInvite struct {
	InviteID      string `json:"inviteId"`
	WorkspaceID   string `json:"workspaceId"`
	InviterUserID string `json:"inviterUserId"`
	InviteeUserID string `json:"inviteeUserId,omitempty"`
	InviteeEmail  string `json:"inviteeEmail"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"createdAt"`
	HandledAt     int64  `json:"handledAt,omitempty"`
}

type WorkspaceDeviceInvite struct {
	InviteID         string `json:"inviteId"`
	WorkspaceID      string `json:"workspaceId,omitempty"`
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

type WorkspaceDevice struct {
	WorkspaceDeviceID string `json:"workspaceDeviceId"`
	WorkspaceID       string `json:"workspaceId"`
	DeviceID          string `json:"deviceId"`
	OwnerUserID       string `json:"ownerUserId"`
	Alias             string `json:"alias,omitempty"`
	Enabled           bool   `json:"enabled"`
	Status            string `json:"status"`
	CreatedAt         int64  `json:"createdAt"`
	UpdatedAt         int64  `json:"updatedAt"`
}

type WorkspaceDNSZone struct {
	ZoneID       string `json:"zoneId"`
	WorkspaceID  string `json:"workspaceId"`
	ZoneName     string `json:"zoneName"`
	ExposeGlobal bool   `json:"exposeGlobal"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
}

type WorkspaceDNSRecord struct {
	RecordID       string `json:"recordId"`
	ZoneID         string `json:"zoneId"`
	WorkspaceID    string `json:"workspaceId"`
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
	WorkspaceID  string `json:"workspaceId"`
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
	WorkspaceID     string `json:"workspaceId"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	DefaultPolicy   string `json:"defaultPolicy"`
	Status          string `json:"status"`
	CreatedAt       int64  `json:"createdAt"`
}

type SecurityGroupDevice struct {
	SecurityGroupID string `json:"securityGroupId"`
	DeviceID        string `json:"deviceId"`
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
	WorkspaceID   string `json:"workspaceId"`
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
	WorkspaceID    string               `json:"workspaceId"`
	DeviceID       string               `json:"deviceId"`
	GlobalIP       string               `json:"globalIp"`
	GlobalName     string               `json:"globalName"`
	Peers          []Device             `json:"peers"`
	SecurityGroups []SecurityGroup      `json:"securityGroups"`
	Rules          []SecurityGroupRule  `json:"rules"`
	DNSZones       []WorkspaceDNSZone   `json:"dnsZones"`
	DNSRecords     []WorkspaceDNSRecord `json:"dnsRecords"`
}
