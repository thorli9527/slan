package repository

type gormDeviceRecord struct {
	DeviceID      string `gorm:"primaryKey;size:64"`
	VirtualIP     string `gorm:"size:64;index"`
	Name          string `gorm:"size:255"`
	Platform      string `gorm:"size:64"`
	Alias         string `gorm:"size:255"`
	OSName        string `gorm:"size:255"`
	OSVersion     string `gorm:"size:255"`
	PublicKey     string `gorm:"size:1024"`
	DeviceVersion string `gorm:"size:255"`
	CountryCode   string `gorm:"size:32;index"`
	RXBytesTotal  int64  `gorm:"not null"`
	TXBytesTotal  int64  `gorm:"not null"`
	Status        string `gorm:"size:64;index"`
	CreatedAt     int64  `gorm:"not null"`
	UpdatedAt     int64  `gorm:"not null"`
	LastSeenAt    int64  `gorm:"not null"`
}

type gormDeviceUserRelationRecord struct {
	RelationID string `gorm:"primaryKey;size:64"`
	DeviceID   string `gorm:"size:64;uniqueIndex:uidx_device_user_relation;index"`
	UserID     string `gorm:"size:64;uniqueIndex:uidx_device_user_relation;index"`
	Role       string `gorm:"size:32;index"`
	SourceType string `gorm:"size:32;index"`
	SourceID   string `gorm:"size:64;index"`
	Status     string `gorm:"size:32;index"`
	CreatedBy  string `gorm:"size:64"`
	CreatedAt  int64  `gorm:"not null"`
	UpdatedAt  int64  `gorm:"not null"`
	RevokedBy  string `gorm:"size:64"`
	RevokedAt  int64  `gorm:"not null"`
}

type gormDeviceLoginRecord struct {
	DeviceID      string `gorm:"primaryKey;size:64"`
	UserID        string `gorm:"size:64;index"`
	Name          string `gorm:"size:255"`
	Platform      string `gorm:"size:64"`
	Alias         string `gorm:"size:255"`
	OSName        string `gorm:"size:255"`
	OSVersion     string `gorm:"size:255"`
	PublicKey     string `gorm:"size:1024"`
	DeviceVersion string `gorm:"size:255"`
	CountryCode   string `gorm:"size:32;index"`
	VerifyCode    string `gorm:"size:128;index"`
	Status        string `gorm:"size:64;index"`
	ExpiresAt     int64  `gorm:"not null"`
	CreatedAt     int64  `gorm:"not null"`
	UpdatedAt     int64  `gorm:"not null"`
}

type gormDeviceSessionRecord struct {
	SessionID     string `gorm:"primaryKey;size:64"`
	DeviceID      string `gorm:"size:64;index"`
	AccessToken   string `gorm:"size:255;index"`
	RefreshToken  string `gorm:"size:255"`
	Status        string `gorm:"size:64;index"`
	SessionMode   string `gorm:"size:64;index"`
	ExpiresAt     int64  `gorm:"not null"`
	RefreshExpiry int64  `gorm:"not null"`
	CreatedAt     int64  `gorm:"not null"`
	UpdatedAt     int64  `gorm:"not null"`
	RevokedAt     int64  `gorm:"not null"`
}

type gormBootstrapKeyRecord struct {
	KeyID          string `gorm:"primaryKey;size:64"`
	UserID         string `gorm:"size:64;index"`
	NetworkID      string `gorm:"size:64;index"`
	Name           string `gorm:"size:255"`
	Token          string `gorm:"size:255;index"`
	Status         string `gorm:"size:64;index"`
	ExpiresAt      int64  `gorm:"not null"`
	UsedAt         int64  `gorm:"not null"`
	UsedByDeviceID string `gorm:"size:64;index"`
	CreatedAt      int64  `gorm:"not null"`
	UpdatedAt      int64  `gorm:"not null"`
	RevokedAt      int64  `gorm:"not null"`
}

type gormDeviceGroupRecord struct {
	GroupID     string `gorm:"primaryKey;size:64"`
	UserID      string `gorm:"size:64;index"`
	Name        string `gorm:"size:255"`
	Description string `gorm:"size:255"`
	CreatedAt   int64  `gorm:"not null"`
	UpdatedAt   int64  `gorm:"not null"`
}

type gormDeviceGroupAssignmentRecord struct {
	DeviceID  string          `gorm:"primaryKey;size:64"`
	UserID    string          `gorm:"size:64;index"`
	GroupIDs  jsonStringSlice `gorm:"type:json"`
	UpdatedAt int64           `gorm:"not null"`
}
