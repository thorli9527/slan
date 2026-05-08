package repo

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
)

type Device struct {
	DeviceID      string  `gorm:"column:device_id;primaryKey"`
	UserID        string  `gorm:"column:user_id;->;-:migration"`
	Name          string  `gorm:"column:name;not null"`
	Platform      string  `gorm:"column:platform;not null"`
	DeviceVersion string  `gorm:"column:device_version;not null;default:''"`
	CountryCode   string  `gorm:"column:country_code;not null;default:''"`
	Status        string  `gorm:"column:status;not null"`
	PublicKey     *string `gorm:"column:public_key"`
	CreatedAt     int64   `gorm:"column:created_at;not null;default:0"`
}

func (Device) TableName() string { return "devices" }

type DeviceUser struct {
	DeviceID string `gorm:"column:device_id;primaryKey"`
	UserID   string `gorm:"column:user_id;primaryKey;index;not null"`
	BoundAt  int64  `gorm:"column:bound_at;not null;default:0"`
}

func (DeviceUser) TableName() string { return "device_users" }

type DeviceInstallRegistration struct {
	DeviceID     string `gorm:"column:device_id;primaryKey"`
	IPAddress    string `gorm:"column:ip_address;primaryKey"`
	RegisterDay  string `gorm:"column:register_day;primaryKey"`
	RegisteredAt int64  `gorm:"column:registered_at;not null;default:0"`
}

func (DeviceInstallRegistration) TableName() string { return "device_install_registrations" }

func (m Device) ToDTO(networkIDs []string) dto.Device {
	var publicKey string
	if m.PublicKey != nil {
		publicKey = *m.PublicKey
	}
	return dto.Device{
		DeviceID:      m.DeviceID,
		Name:          m.Name,
		Platform:      m.Platform,
		DeviceVersion: m.DeviceVersion,
		CountryCode:   m.CountryCode,
		Status:        m.Status,
		CreatedAt:     m.CreatedAt,
		PublicKey:     publicKey,
		NetworkIDs:    networkIDs,
	}
}

func (r *PostgresRepository) InsertDevice(ctx context.Context, record Device) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) UpdateDevice(ctx context.Context, record Device) error {
	return r.db.WithContext(ctx).
		Model(&Device{}).
		Where("device_id = ?", record.DeviceID).
		Updates(map[string]any{
			"name":           record.Name,
			"platform":       record.Platform,
			"device_version": record.DeviceVersion,
			"country_code":   record.CountryCode,
			"status":         record.Status,
			"public_key":     record.PublicKey,
		}).Error
}

func (r *PostgresRepository) ListDevicesByUser(ctx context.Context, userID string) ([]Device, error) {
	var out []Device
	err := r.db.WithContext(ctx).
		Table("devices").
		Select("devices.*, device_users.user_id AS user_id").
		Joins("join device_users on device_users.device_id = devices.device_id").
		Where("device_users.user_id = ?", userID).
		Order("devices.device_id").
		Find(&out).Error
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
	err := r.db.WithContext(ctx).
		Table("devices").
		Select("devices.*, COALESCE(device_users.user_id, '') AS user_id").
		Joins("left join device_users on device_users.device_id = devices.device_id").
		Order("devices.device_id").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) GetDeviceByID(ctx context.Context, deviceID string) (Device, error) {
	var record Device
	err := r.db.WithContext(ctx).
		Table("devices").
		Select("devices.*, COALESCE(device_users.user_id, '') AS user_id").
		Joins("left join device_users on device_users.device_id = devices.device_id").
		Where("devices.device_id = ?", deviceID).
		Order("device_users.bound_at desc").
		First(&record).Error
	return record, err
}

func (r *PostgresRepository) BindDeviceUser(ctx context.Context, deviceID, userID string, boundAt int64) error {
	return r.db.WithContext(ctx).Save(&DeviceUser{
		DeviceID: deviceID,
		UserID:   userID,
		BoundAt:  boundAt,
	}).Error
}

func (r *PostgresRepository) GetDeviceUser(ctx context.Context, deviceID, userID string) (DeviceUser, error) {
	var record DeviceUser
	err := r.db.WithContext(ctx).Where("device_id = ? AND user_id = ?", deviceID, userID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) CountInstallRegisteredDevicesByIPDay(ctx context.Context, ipAddress, registerDay string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&DeviceInstallRegistration{}).
		Where("ip_address = ? AND register_day = ?", ipAddress, registerDay).
		Count(&count).Error
	return count, err
}

func (r *PostgresRepository) GetInstallRegistration(ctx context.Context, deviceID, ipAddress, registerDay string) (DeviceInstallRegistration, error) {
	var record DeviceInstallRegistration
	err := r.db.WithContext(ctx).
		Where("device_id = ? AND ip_address = ? AND register_day = ?", deviceID, ipAddress, registerDay).
		First(&record).Error
	return record, err
}

func (r *PostgresRepository) InsertInstallRegistration(ctx context.Context, record DeviceInstallRegistration) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) UpdateDeviceStatus(ctx context.Context, deviceID, status string) error {
	return r.db.WithContext(ctx).
		Model(&Device{}).
		Where("device_id = ?", deviceID).
		Update("status", status).Error
}
