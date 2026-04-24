package repo

import "context"

// ControlSession 持久化保存一个节点在某个网络下的控制面会话。
type ControlSession struct {
	// ControlSessionID 是会话唯一标识。
	ControlSessionID string `gorm:"column:control_session_id;primaryKey"`
	// UserID 是会话所属用户。
	UserID string `gorm:"column:user_id;index;not null"`
	// DeviceID 是会话所属设备。
	DeviceID string `gorm:"column:device_id;index;not null"`
	// NodeID 是会话所属节点。
	NodeID string `gorm:"column:node_id;index;not null"`
	// NetworkID 是该会话绑定的逻辑网络。
	NetworkID string `gorm:"column:network_id;index;not null"`
	// SessionToken 是控制面使用的鉴权令牌。
	SessionToken string `gorm:"column:session_token;uniqueIndex;not null"`
	// ConnectedAt 是会话建立时间。
	ConnectedAt int64 `gorm:"column:connected_at;not null"`
	// LastSeenAt 是最近一次活跃时间。
	LastSeenAt int64 `gorm:"column:last_seen_at;index;not null"`
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

func (r *PostgresRepository) TouchControlSessionByToken(ctx context.Context, sessionToken string, lastSeenAt int64) error {
	return r.db.WithContext(ctx).
		Model(&ControlSession{}).
		Where("session_token = ?", sessionToken).
		Update("last_seen_at", lastSeenAt).Error
}

func (r *PostgresRepository) TouchControlSessionByNode(ctx context.Context, nodeID, networkID string, lastSeenAt int64) error {
	return r.db.WithContext(ctx).
		Model(&ControlSession{}).
		Where("node_id = ? AND network_id = ?", nodeID, networkID).
		Update("last_seen_at", lastSeenAt).Error
}

func (r *PostgresRepository) GetLatestControlSessionByNode(ctx context.Context, nodeID, networkID string) (ControlSession, error) {
	var record ControlSession
	err := r.db.WithContext(ctx).
		Where("node_id = ? AND network_id = ?", nodeID, networkID).
		Order("last_seen_at desc, connected_at desc").
		First(&record).Error
	return record, err
}

func (r *PostgresRepository) GetLatestControlSessionByDevice(ctx context.Context, deviceID string) (ControlSession, error) {
	var record ControlSession
	err := r.db.WithContext(ctx).
		Where("device_id = ?", deviceID).
		Order("last_seen_at desc, connected_at desc").
		First(&record).Error
	return record, err
}

func (r *PostgresRepository) DeleteControlSessionByNode(ctx context.Context, nodeID, networkID string) error {
	return r.db.WithContext(ctx).
		Where("node_id = ? AND network_id = ?", nodeID, networkID).
		Delete(&ControlSession{}).Error
}

func (r *PostgresRepository) DeleteControlSessionsBefore(ctx context.Context, cutoff int64) error {
	return r.db.WithContext(ctx).
		Where("last_seen_at < ?", cutoff).
		Delete(&ControlSession{}).Error
}

func (r *PostgresRepository) MarkDevicesOfflineWithoutFreshControlSession(ctx context.Context, cutoff int64) error {
	return r.db.WithContext(ctx).
		Model(&Device{}).
		Where("status = ?", "online").
		Where("NOT EXISTS (?)",
			r.db.Model(&ControlSession{}).
				Select("1").
				Where("control_sessions.device_id = devices.device_id").
				Where("control_sessions.last_seen_at >= ?", cutoff),
		).
		Update("status", "offline").Error
}

func (r *PostgresRepository) DeleteControlSessionsByDeviceExceptNetwork(ctx context.Context, deviceID, keepNetworkID string) error {
	query := r.db.WithContext(ctx).Where("device_id = ?", deviceID)
	if keepNetworkID != "" {
		query = query.Where("network_id <> ?", keepNetworkID)
	}
	return query.Delete(&ControlSession{}).Error
}
