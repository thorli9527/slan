package repo

import "context"

type ControlSession struct {
	ControlSessionID string `gorm:"column:control_session_id;primaryKey"`
	UserID           string `gorm:"column:user_id;index;not null"`
	DeviceID         string `gorm:"column:device_id;index;not null"`
	NodeID           string `gorm:"column:node_id;index;not null"`
	NetworkID        string `gorm:"column:network_id;index;not null"`
	SessionToken     string `gorm:"column:session_token;uniqueIndex;not null"`
}

func (ControlSession) TableName() string { return "control_sessions" }

func (r *PostgresRepository) CreateControlSession(ctx context.Context, session ControlSession) error {
	return r.db.WithContext(ctx).Create(&session).Error
}

func (r *PostgresRepository) GetControlSessionByToken(ctx context.Context, sessionToken string) (ControlSession, error) {
	var record ControlSession
	err := r.db.WithContext(ctx).Where("session_token = ?", sessionToken).First(&record).Error
	return record, err
}
