package repository

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
)

type DeviceRuntimeRepository interface {
	GetDeviceRuntime(ctx context.Context, deviceID string) (model.DeviceRuntimeState, bool, error)
	RefreshDeviceHeartbeat(ctx context.Context, state model.DeviceRuntimeState, ttl time.Duration) error
	RefreshDeviceNetworkState(ctx context.Context, state model.DeviceRuntimeState, ttl time.Duration) error
}
