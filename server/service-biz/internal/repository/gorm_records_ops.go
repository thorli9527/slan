package repository

type gormOperatorRecord struct {
	OperatorID   string `gorm:"primaryKey;size:64"`
	Email        string `gorm:"size:255;index"`
	Name         string `gorm:"size:255"`
	PasswordHash string `gorm:"size:255"`
	Role         string `gorm:"size:64;index"`
	Status       string `gorm:"size:64;index"`
	CreatedAt    int64  `gorm:"not null"`
	UpdatedAt    int64  `gorm:"not null"`
}

type gormOperatorSessionRecord struct {
	SessionID   string `gorm:"primaryKey;size:64"`
	OperatorID  string `gorm:"size:64;index"`
	AccessToken string `gorm:"size:255;index"`
	ExpiresAt   int64  `gorm:"not null"`
	CreatedAt   int64  `gorm:"not null"`
}

type gormAuditEventRecord struct {
	EventID      string `gorm:"primaryKey;size:64"`
	ActorType    string `gorm:"size:64;index"`
	ActorID      string `gorm:"size:64;index"`
	Action       string `gorm:"size:128;index"`
	ResourceType string `gorm:"size:64;index"`
	ResourceID   string `gorm:"size:64;index"`
	Status       string `gorm:"size:64;index"`
	CreatedAt    int64  `gorm:"not null"`
}

type gormRelayNodeRecord struct {
	NodeID           string `gorm:"primaryKey;size:64"`
	Name             string `gorm:"size:255"`
	Region           string `gorm:"size:64;index"`
	Endpoint         string `gorm:"size:255"`
	Transport        string `gorm:"size:64"`
	Priority         int    `gorm:"not null"`
	TicketKeySource          string `gorm:"size:64"`
	TicketKeyRingID          string `gorm:"size:128"`
	TicketSigningConfigured  bool   `gorm:"not null"`
	TicketKeyRingConfigured  bool   `gorm:"not null"`
	TicketEffectiveKeyCount  int    `gorm:"not null"`
	TicketRotationReady      bool   `gorm:"not null"`
	TicketAcceptsDevFallback bool   `gorm:"not null"`
	MaxBandwidthMbps int    `gorm:"not null"`
	MonthlyTrafficGb int    `gorm:"not null"`
	UsedTrafficGb    int    `gorm:"not null"`
	MaxSessions      int    `gorm:"not null"`
	ActiveSessions   int    `gorm:"not null"`
	Status           string `gorm:"size:64;index"`
	Health           string `gorm:"size:64"`
	CreatedAt        int64  `gorm:"not null"`
	UpdatedAt        int64  `gorm:"not null"`
}

type gormPunchNodeRecord struct {
	NodeID         string `gorm:"primaryKey;size:64"`
	Name           string `gorm:"size:255"`
	Region         string `gorm:"size:64;index"`
	Endpoint       string `gorm:"size:255"`
	MaxSessions    int    `gorm:"not null"`
	ActiveSessions int    `gorm:"not null"`
	Status         string `gorm:"size:64;index"`
	Health         string `gorm:"size:64"`
	Priority       int    `gorm:"not null"`
	CreatedAt      int64  `gorm:"not null"`
	UpdatedAt      int64  `gorm:"not null"`
}

type gormCustomerPlanRecord struct {
	CustomerID string `gorm:"primaryKey;size:64"`
	PlanCode   string `gorm:"size:64;index"`
}

type gormClientDownloadRecord struct {
	DownloadID string `gorm:"primaryKey;size:64"`
	Name       string `gorm:"size:255"`
	Platform   string `gorm:"size:64;index"`
	Version    string `gorm:"size:64"`
	Arch       string `gorm:"size:64"`
	Channel    string `gorm:"size:64"`
	URL        string `gorm:"size:255"`
	SHA256     string `gorm:"size:255"`
	ReleaseNotes string `gorm:"type:text"`
	Status     string `gorm:"size:64;index"`
	CreatedAt  int64  `gorm:"not null"`
	UpdatedAt  int64  `gorm:"not null"`
}

type gormPlanRecord struct {
	PlanCode           string `gorm:"primaryKey;size:64"`
	Name               string `gorm:"size:255"`
	DeviceLimit        int    `gorm:"not null"`
	InvitedDeviceLimit int    `gorm:"not null"`
	TotalDeviceLimit   int    `gorm:"not null"`
	RelayMonthlyGb     int    `gorm:"not null"`
	RelayBandwidthMbps int    `gorm:"not null"`
	RelayThrottleMbps  int    `gorm:"not null"`
	P2PUnlimited       bool   `gorm:"not null"`
	CustomDomain       bool   `gorm:"not null"`
	Acl                bool   `gorm:"not null"`
	DedicatedRelay     bool   `gorm:"not null"`
	AuditLog           bool   `gorm:"not null"`
	ApiAccess          bool   `gorm:"not null"`
	MonthlyPrice       int64  `gorm:"not null"`
	YearlyPrice        int64  `gorm:"not null"`
	Status             string `gorm:"size:64;index"`
	UpdatedAt          int64  `gorm:"not null"`
}

type gormProductRecord struct {
	ProductID          string `gorm:"primaryKey;size:64"`
	Name               string `gorm:"size:255"`
	Type               string `gorm:"size:64;index"`
	PlanCode           string `gorm:"size:64;index"`
	Period             string `gorm:"size:64"`
	ValidDays          int    `gorm:"not null"`
	RelayTrafficGb     int    `gorm:"not null"`
	RelayBandwidthMbps int    `gorm:"not null"`
	Price              int64  `gorm:"not null"`
	SalePrice          int64  `gorm:"not null"`
	Currency           string `gorm:"size:16"`
	AutoRenew          bool   `gorm:"not null"`
	Status             string `gorm:"size:64;index"`
	Description        string `gorm:"type:text"`
	CreatedAt          int64  `gorm:"not null"`
	UpdatedAt          int64  `gorm:"not null"`
}

type gormOrderRecord struct {
	OrderID          string `gorm:"primaryKey;size:64"`
	CustomerID       string `gorm:"size:64;index"`
	CustomerEmail    string `gorm:"size:255;index"`
	ProductID        string `gorm:"size:64;index"`
	ProductName      string `gorm:"size:255"`
	ProductType      string `gorm:"size:64"`
	Status           string `gorm:"size:64;index"`
	Amount           int64  `gorm:"not null"`
	Currency         string `gorm:"size:16"`
	PayStatus        string `gorm:"size:64;index"`
	ProvisionStatus  string `gorm:"size:64;index"`
	Channel          string `gorm:"size:64"`
	PaidAt           int64  `gorm:"not null"`
	ValidUntil       int64  `gorm:"not null"`
	CreatedAt        int64  `gorm:"not null"`
	UpdatedAt        int64  `gorm:"not null"`
}

type gormRenewalRecord struct {
	RenewalID     string `gorm:"primaryKey;size:64"`
	OrderID       string `gorm:"size:64;index"`
	CustomerID    string `gorm:"size:64;index"`
	CustomerEmail string `gorm:"size:255;index"`
	PlanCode      string `gorm:"size:64;index"`
	Period        string `gorm:"size:64"`
	Amount        int64  `gorm:"not null"`
	Status        string `gorm:"size:64;index"`
	RenewAt       int64  `gorm:"not null"`
	PaidAt        int64  `gorm:"not null"`
	Source        string `gorm:"size:64"`
	Operator      string `gorm:"size:255"`
	UpdatedAt     int64  `gorm:"not null"`
}
