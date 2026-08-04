package repository

import (
	"context"
	"github.com/slan/service-biz/internal/model"
)

type DeviceInventoryRepository interface {
	ListAllDevices(ctx context.Context) ([]model.Device, error)
}
