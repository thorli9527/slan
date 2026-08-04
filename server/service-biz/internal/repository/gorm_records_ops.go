package repository

type gormOperatorRecord struct {
	OperatorID   string `gorm:"primaryKey;size:64"`
	Email        string `gorm:"size:255;uniqueIndex"`
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
	AccessToken string `gorm:"size:255;uniqueIndex"`
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
	RemoteIP     string `gorm:"size:128;index"`
	Detail       string `gorm:"size:512"`
	CreatedAt    int64  `gorm:"not null;index"`
}

type gormRelayNodeRecord struct {
	NodeID                   string `gorm:"primaryKey;size:64"`
	Name                     string `gorm:"size:255"`
	Region                   string `gorm:"size:64;index"`
	Endpoint                 string `gorm:"size:255"`
	Transport                string `gorm:"size:64"`
	Priority                 int    `gorm:"not null"`
	TicketKeySource          string `gorm:"size:64"`
	TicketKeyRingID          string `gorm:"size:128"`
	TicketSigningConfigured  bool   `gorm:"not null"`
	TicketKeyRingConfigured  bool   `gorm:"not null"`
	TicketEffectiveKeyCount  int    `gorm:"not null"`
	TicketRotationReady      bool   `gorm:"not null"`
	TicketAcceptsDevFallback bool   `gorm:"not null"`
	MaxBandwidthMbps         int    `gorm:"not null"`
	MonthlyTrafficGb         int    `gorm:"not null"`
	UsedTrafficGb            int    `gorm:"not null"`
	MaxSessions              int    `gorm:"not null"`
	ActiveSessions           int    `gorm:"not null"`
	Status                   string `gorm:"size:64;index"`
	Health                   string `gorm:"size:64"`
	CreatedAt                int64  `gorm:"not null"`
	UpdatedAt                int64  `gorm:"not null"`
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
