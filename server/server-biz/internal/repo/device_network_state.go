package repo

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
	"gorm.io/gorm/clause"
)

type DeviceNetworkState struct {
	DeviceID         string `gorm:"column:device_id;primaryKey"`
	NetworkID        string `gorm:"column:network_id;primaryKey"`
	ControlReachable bool   `gorm:"column:control_reachable;not null;default:false"`
	NetworkOnline    bool   `gorm:"column:network_online;index;not null;default:false"`
	TunnelUp         bool   `gorm:"column:tunnel_up;not null;default:false"`
	LastProbeOK      bool   `gorm:"column:last_probe_ok;not null;default:false"`
	VirtualIP        string `gorm:"column:virtual_ip;not null;default:''"`
	LastSeenAt       int64  `gorm:"column:last_seen_at;index;not null;default:0"`
	UpdatedAt        int64  `gorm:"column:updated_at;not null;default:0"`
}

func (DeviceNetworkState) TableName() string { return "device_network_states" }

func (s DeviceNetworkState) ToDTO() dto.DeviceNetworkState {
	return dto.DeviceNetworkState{
		DeviceID:         s.DeviceID,
		NetworkID:        s.NetworkID,
		ControlReachable: s.ControlReachable,
		NetworkOnline:    s.NetworkOnline,
		TunnelUp:         s.TunnelUp,
		LastProbeOK:      s.LastProbeOK,
		VirtualIP:        s.VirtualIP,
		LastSeenAt:       s.LastSeenAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

func (r *PostgresRepository) UpsertDeviceNetworkState(ctx context.Context, record DeviceNetworkState) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "device_id"},
				{Name: "network_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"control_reachable": record.ControlReachable,
				"network_online":    record.NetworkOnline,
				"tunnel_up":         record.TunnelUp,
				"last_probe_ok":     record.LastProbeOK,
				"virtual_ip":        record.VirtualIP,
				"last_seen_at":      record.LastSeenAt,
				"updated_at":        record.UpdatedAt,
			}),
		}).
		Create(&record).Error
}

func (r *PostgresRepository) GetDeviceNetworkState(ctx context.Context, deviceID, networkID string) (DeviceNetworkState, error) {
	var record DeviceNetworkState
	err := r.db.WithContext(ctx).
		Where("device_id = ? AND network_id = ?", deviceID, networkID).
		First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListFreshOnlineDeviceNetworkStates(ctx context.Context, cutoff int64) ([]DeviceNetworkState, error) {
	var records []DeviceNetworkState
	err := r.db.WithContext(ctx).
		Where("network_online = ?", true).
		Where("last_seen_at >= ?", cutoff).
		Find(&records).Error
	return records, err
}

func (r *PostgresRepository) MarkStaleDeviceNetworkStatesOffline(ctx context.Context, cutoff int64, updatedAt int64) error {
	return r.db.WithContext(ctx).
		Model(&DeviceNetworkState{}).
		Where("control_reachable = ? OR network_online = ?", true, true).
		Where("last_seen_at < ?", cutoff).
		Updates(map[string]any{
			"control_reachable": false,
			"network_online":    false,
			"tunnel_up":         false,
			"last_probe_ok":     false,
			"updated_at":        updatedAt,
		}).Error
}
