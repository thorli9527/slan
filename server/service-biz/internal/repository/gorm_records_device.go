package repository

type gormDeviceRecord struct {
	DeviceID      string `gorm:"primaryKey;size:64"`
	VirtualIP     string `gorm:"size:64;uniqueIndex:uidx_devices_virtual_ip,where:virtual_ip <> ''"`
	Name          string `gorm:"size:255"`
	Platform      string `gorm:"size:64"`
	Alias         string `gorm:"size:255"`
	OSName        string `gorm:"size:255"`
	OSVersion     string `gorm:"size:255"`
	PublicKey     string `gorm:"size:1024"`
	DeviceVersion string `gorm:"size:255"`
	PublicIP      string `gorm:"size:128;index"`
	CountryCode   string `gorm:"size:32;index"`
	CityCode      string `gorm:"size:32;index"`
	GeoUpdatedAt  int64  `gorm:"not null;default:0"`
	RXBytesTotal  int64  `gorm:"not null"`
	TXBytesTotal  int64  `gorm:"not null"`
	Status        string `gorm:"size:64;index"`
	CreatedAt     int64  `gorm:"not null"`
	UpdatedAt     int64  `gorm:"not null"`
	LastSeenAt    int64  `gorm:"not null"`
}

type gormDeviceSessionRecord struct {
	SessionID                  string `gorm:"primaryKey;size:64"`
	DeviceID                   string `gorm:"size:64;uniqueIndex"`
	CredentialID               string `gorm:"size:64;index"`
	AccessToken                string `gorm:"size:255;index"`
	RefreshToken               string `gorm:"size:255"`
	Status                     string `gorm:"size:64;index"`
	SessionMode                string `gorm:"size:64;index"`
	PreviousRefreshTokenHash   string `gorm:"size:64;index"`
	RefreshRotationGraceExpiry int64  `gorm:"not null;default:0"`
	ExpiresAt                  int64  `gorm:"not null"`
	RefreshExpiry              int64  `gorm:"not null"`
	CreatedAt                  int64  `gorm:"not null"`
	UpdatedAt                  int64  `gorm:"not null"`
	RevokedAt                  int64  `gorm:"not null"`
}

type gormDeviceCredentialRecord struct {
	CredentialID       string `gorm:"primaryKey;size:64"`
	KeyID              string `gorm:"size:64;uniqueIndex"`
	DeviceID           string `gorm:"size:64;index"`
	Name               string `gorm:"size:255"`
	SecretHash         string `gorm:"size:64"`
	Status             string `gorm:"size:32;index"`
	Scopes             string `gorm:"size:512"`
	LastUsedAt         int64  `gorm:"not null"`
	LastUsedIP         string `gorm:"size:128"`
	OfflineAckAt       int64  `gorm:"not null;default:0;index"`
	DisableNotifiedAt  int64  `gorm:"not null;default:0"`
	DisableNotifyCount int64  `gorm:"not null;default:0"`
	CreatedAt          int64  `gorm:"not null"`
	UpdatedAt          int64  `gorm:"not null"`
	RevokedAt          int64  `gorm:"not null"`
}

type gormDeviceGroupRecord struct {
	GroupID     string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:255"`
	Description string `gorm:"size:255"`
	CreatedAt   int64  `gorm:"not null"`
	UpdatedAt   int64  `gorm:"not null"`
}

type gormDeviceGroupAssignmentRecord struct {
	DeviceID  string          `gorm:"primaryKey;size:64"`
	GroupIDs  jsonStringSlice `gorm:"type:json"`
	UpdatedAt int64           `gorm:"not null"`
}
