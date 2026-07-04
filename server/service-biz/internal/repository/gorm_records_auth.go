package repository

type gormUserRecord struct {
	UserID       string `gorm:"primaryKey;size:64"`
	Email        string `gorm:"size:255;index"`
	Name         string `gorm:"size:255"`
	Country      string `gorm:"size:128"`
	Province     string `gorm:"size:128"`
	City         string `gorm:"size:128"`
	IPRegion     string `gorm:"size:128"`
	PasswordHash string `gorm:"size:255"`
	Status       string `gorm:"size:64;index"`
	CreatedAt    int64  `gorm:"not null"`
	UpdatedAt    int64  `gorm:"not null"`
}

type gormUserSessionRecord struct {
	SessionID     string `gorm:"primaryKey;size:64"`
	UserID        string `gorm:"size:64;index"`
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

type gormConsoleLoginKeyRecord struct {
	KeyID     string `gorm:"primaryKey;size:64"`
	UserID    string `gorm:"size:64;index"`
	Key       string `gorm:"size:255;index"`
	Status    string `gorm:"size:64;index"`
	ExpiresAt int64  `gorm:"not null"`
	CreatedAt int64  `gorm:"not null"`
	UpdatedAt int64  `gorm:"not null"`
}

type gormUserAliasRecord struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	UserID    string `gorm:"size:64;uniqueIndex:uidx_gorm_user_alias_records_user_alias"`
	Email     string `gorm:"size:255;index"`
	Alias     string `gorm:"size:255;uniqueIndex:uidx_gorm_user_alias_records_user_alias"`
	CreatedAt int64  `gorm:"not null"`
	UpdatedAt int64  `gorm:"not null"`
}
