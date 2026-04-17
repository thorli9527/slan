package repo

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
)

type Device struct {
	DeviceID  string  `gorm:"column:device_id;primaryKey"`
	UserID    string  `gorm:"column:user_id;index;not null"`
	MachineID string  `gorm:"column:machine_id;not null;uniqueIndex:idx_user_machine"`
	Name      string  `gorm:"column:name;not null"`
	Platform  string  `gorm:"column:platform;not null"`
	Status    string  `gorm:"column:status;not null"`
	PublicKey *string `gorm:"column:public_key"`
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
		Status:     m.Status,
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
