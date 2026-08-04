package service

import (
	"context"

	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type DeviceCoreUseCase interface {
	DeviceCatalogUseCase
	DeviceRuntimeAccessUseCase
}

type DeviceCatalogUseCase interface {
	GetDeviceProfile(ctx context.Context, deviceID string) (DeviceProfileView, error)
}

type DeviceRuntimeAccessUseCase interface {
	RenewDevice(ctx context.Context, deviceID string) (DeviceProfileView, error)
	UpdateDeviceRuntime(ctx context.Context, input UpdateDeviceRuntimeInput) (DeviceProfileView, error)
	DeviceNetworkConfigs(ctx context.Context, deviceID string) ([]NetworkSummaryView, error)
	DeviceMQTTCredential(ctx context.Context, deviceID, credentialID string, expiresAt int64) (*mqttkit.Credential, error)
	DeviceMQTTProfile(ctx context.Context, deviceID, credentialID string, expiresAt int64) (DeviceMQTTProfileView, error)
}
