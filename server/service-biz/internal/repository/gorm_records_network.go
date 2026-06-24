package repository

type gormNetworkRecord struct {
	NetworkID        string `gorm:"primaryKey;size:64"`
	OwnerID          string `gorm:"size:64;index"`
	Name             string `gorm:"size:255"`
	CIDR             string `gorm:"column:cidr;size:128"`
	Code             string `gorm:"size:128;index"`
	TemplateKey      string `gorm:"size:128"`
	IntraGroupPolicy string `gorm:"size:32"`
	Default          bool   `gorm:"not null"`
	Status           string `gorm:"size:64;index"`
	CreatedAt        int64  `gorm:"not null"`
	UpdatedAt        int64  `gorm:"not null"`
}

type gormNetworkDeviceRecord struct {
	ID              uint64              `gorm:"primaryKey;autoIncrement"`
	NetworkID       string              `gorm:"size:64;index:idx_network_device,unique"`
	DeviceID        string              `gorm:"size:64;index:idx_network_device,unique"`
	Enabled         bool                `gorm:"not null"`
	Status          string              `gorm:"size:64;index"`
	Endpoints       jsonDeviceEndpoints `gorm:"type:json"`
	NATType         string              `gorm:"size:64"`
	ActivePath      string              `gorm:"size:64"`
	PathObservedAt  int64               `gorm:"not null"`
	RelayTransport  string              `gorm:"size:64"`
	RelayEndpoint   string              `gorm:"size:255"`
	DerpNodeID      string              `gorm:"size:64"`
	PeerNodeID      string              `gorm:"size:64"`
	PathScore       int64               `gorm:"not null"`
	ObservedRttMs   int64               `gorm:"not null"`
	PacketLossPpm   int64               `gorm:"not null"`
	RelayMtu        int                 `gorm:"not null"`
	MaxFramePayload int                 `gorm:"not null"`
	TicketExpiresAt string              `gorm:"size:128"`
	TicketRenewDue  bool                `gorm:"not null"`
	PathDowngrades  int64               `gorm:"not null"`
	PathUpgrades    int64               `gorm:"not null"`
	LastPathChange  string              `gorm:"size:128"`
	CreatedAt       int64               `gorm:"not null"`
	UpdatedAt       int64               `gorm:"not null"`
}

type gormDeviceInviteRecord struct {
	InviteID      string `gorm:"primaryKey;size:64"`
	InviteCode    string `gorm:"size:255;index"`
	InviterUserID string `gorm:"size:64;index"`
	NetworkID     string `gorm:"size:64;index"`
	DeviceID      string `gorm:"size:64;index"`
	UserID        string `gorm:"size:64;index"`
	Status        string `gorm:"size:64;index"`
	CreatedAt     int64  `gorm:"not null"`
	ExpiresAt     int64  `gorm:"not null"`
	AcceptedAt    int64  `gorm:"not null"`
}

type gormDNSZoneRecord struct {
	ZoneID       string `gorm:"primaryKey;size:64"`
	NetworkID    string `gorm:"size:64;index"`
	Name         string `gorm:"size:255"`
	ExposeGlobal bool   `gorm:"not null"`
	Status       string `gorm:"size:64;index"`
	CreatedAt    int64  `gorm:"not null"`
	UpdatedAt    int64  `gorm:"not null"`
}

type gormDNSRecordRecord struct {
	RecordID  string `gorm:"primaryKey;size:64"`
	NetworkID string `gorm:"size:64;index"`
	ZoneID    string `gorm:"size:64;index"`
	Name      string `gorm:"size:255"`
	Type      string `gorm:"size:32;index"`
	Value     string `gorm:"size:255"`
	Port      string `gorm:"size:64"`
	TTL       int    `gorm:"not null"`
	CreatedAt int64  `gorm:"not null"`
	UpdatedAt int64  `gorm:"not null"`
}

type gormPublicMappingRecord struct {
	MappingID    string `gorm:"primaryKey;size:64"`
	NetworkID    string `gorm:"size:64;index"`
	Name         string `gorm:"size:255"`
	PublicDomain string `gorm:"size:255"`
	SourceRecord string `gorm:"size:255"`
	DeviceID     string `gorm:"size:64;index"`
	Protocol     string `gorm:"size:32;index"`
	InternalIP   string `gorm:"size:128"`
	InternalPort int    `gorm:"not null"`
	ExternalPort int    `gorm:"not null"`
	AccessMode   string `gorm:"size:32"`
	TLSMode      string `gorm:"size:32"`
	Status       string `gorm:"size:64;index"`
	CreatedAt    int64  `gorm:"not null"`
	UpdatedAt    int64  `gorm:"not null"`
}

type gormSecurityGroupRecord struct {
	SecurityGroupID string `gorm:"primaryKey;size:64"`
	NetworkID       string `gorm:"size:64;index"`
	Name            string `gorm:"size:255"`
	Description     string `gorm:"size:255"`
	CreatedAt       int64  `gorm:"not null"`
	UpdatedAt       int64  `gorm:"not null"`
}

type gormSecurityRuleRecord struct {
	RuleID          string `gorm:"primaryKey;size:64"`
	SecurityGroupID string `gorm:"size:64;index"`
	Direction       string `gorm:"size:32;index"`
	Protocol        string `gorm:"size:32;index"`
	PortRange       string `gorm:"size:64"`
	CIDR            string `gorm:"column:cidr;size:128"`
	Action          string `gorm:"size:32;index"`
	Priority        int    `gorm:"not null"`
	Description     string `gorm:"size:255"`
	Enabled         bool   `gorm:"not null"`
	CreatedAt       int64  `gorm:"not null"`
	UpdatedAt       int64  `gorm:"not null"`
}
