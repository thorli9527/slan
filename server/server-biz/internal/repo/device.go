package repo

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
	"gorm.io/gorm/clause"
)

// Device 是业务设备的持久化模型。
type Device struct {
	// DeviceID 是设备唯一标识。
	DeviceID string `gorm:"column:device_id;primaryKey"`
	// UserID 是设备所属用户。
	UserID string `gorm:"column:user_id;index;not null;uniqueIndex:idx_user_machine"`
	// MachineID 是设备安装实例的机器标识。
	MachineID string `gorm:"column:machine_id;not null;uniqueIndex:idx_user_machine"`
	// Name 是设备展示名称。
	Name string `gorm:"column:name;not null"`
	// Platform 是设备平台，例如 macos 或 linux。
	Platform string `gorm:"column:platform;not null"`
	// Status 是设备状态。
	Status string `gorm:"column:status;not null"`
	// PublicKey 是设备隧道公钥。
	PublicKey *string `gorm:"column:public_key"`
	// CreatedAt 是设备创建时间。
	CreatedAt int64 `gorm:"column:created_at;not null;default:0"`
}

func (Device) TableName() string { return "devices" }

func (m Device) ToDTO(networkIDs []string) dto.Device {
	var publicKey string
	if m.PublicKey != nil {
		publicKey = *m.PublicKey
	}
	return dto.Device{
		DeviceID:   m.DeviceID,
		Name:       m.Name,
		Platform:   m.Platform,
		MachineID:  m.MachineID,
		Status:     m.Status,
		CreatedAt:  m.CreatedAt,
		PublicKey:  publicKey,
		NetworkIDs: networkIDs,
	}
}

func (r *PostgresRepository) GetDeviceByUserMachine(ctx context.Context, userID, machineID string) (Device, error) {
	var record Device
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND machine_id = ?", userID, machineID).
		First(&record).Error
	return record, err
}

func (r *PostgresRepository) InsertDevice(ctx context.Context, record Device) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) UpsertDeviceByUserMachine(ctx context.Context, record Device) (Device, error) {
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "machine_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"name":       record.Name,
				"platform":   record.Platform,
				"status":     record.Status,
				"public_key": record.PublicKey,
			}),
		}).
		Create(&record).Error; err != nil {
		return Device{}, err
	}
	return r.GetDeviceByUserMachine(ctx, record.UserID, record.MachineID)
}

func (r *PostgresRepository) UpdateDevice(ctx context.Context, record Device) error {
	return r.db.WithContext(ctx).
		Model(&Device{}).
		Where("device_id = ?", record.DeviceID).
		Updates(map[string]any{
			"name":       record.Name,
			"platform":   record.Platform,
			"status":     record.Status,
			"public_key": record.PublicKey,
		}).Error
}

func (r *PostgresRepository) ListDevicesByUser(ctx context.Context, userID string) ([]Device, error) {
	var out []Device
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("device_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListDevicesByNetworks(ctx context.Context, networkIDs []string) ([]Device, error) {
	if len(networkIDs) == 0 {
		return []Device{}, nil
	}
	var out []Device
	err := r.db.WithContext(ctx).
		Table("devices").
		Select("distinct devices.*").
		Joins("join network_members on network_members.device_id = devices.device_id").
		Where("network_members.network_id IN ?", networkIDs).
		Order("devices.device_id").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListDevices(ctx context.Context) ([]Device, error) {
	var out []Device
	err := r.db.WithContext(ctx).Order("device_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) GetDeviceByID(ctx context.Context, deviceID string) (Device, error) {
	var record Device
	err := r.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) UpdateDeviceStatus(ctx context.Context, deviceID, status string) error {
	return r.db.WithContext(ctx).
		Model(&Device{}).
		Where("device_id = ?", deviceID).
		Update("status", status).Error
}
