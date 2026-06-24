package repository

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
)

type GormConfig struct {
	DSN string
}

type GormStore struct {
	db *gorm.DB
}

type gormCounter struct {
	Name  string `gorm:"primaryKey;size:64"`
	Value int64  `gorm:"not null"`
}

type jsonStringSlice []string

type jsonDeviceEndpoints []model.DeviceEndpoint

func (v jsonStringSlice) Value() (driver.Value, error) {
	data, err := json.Marshal([]string(v))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (v *jsonStringSlice) Scan(src any) error {
	switch value := src.(type) {
	case nil:
		*v = nil
		return nil
	case []byte:
		return json.Unmarshal(value, v)
	case string:
		return json.Unmarshal([]byte(value), v)
	default:
		return fmt.Errorf("unsupported jsonStringSlice source %T", src)
	}
}

func (v jsonDeviceEndpoints) Value() (driver.Value, error) {
	data, err := json.Marshal([]model.DeviceEndpoint(v))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (v *jsonDeviceEndpoints) Scan(src any) error {
	switch value := src.(type) {
	case nil:
		*v = nil
		return nil
	case []byte:
		return json.Unmarshal(value, v)
	case string:
		return json.Unmarshal([]byte(value), v)
	default:
		return fmt.Errorf("unsupported jsonDeviceEndpoints source %T", src)
	}
}
