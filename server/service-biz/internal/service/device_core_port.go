package service

import (
	"context"

	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type DeviceCoreUseCase interface {
	DeviceCatalogUseCase
	DeviceProvisioningUseCase
	DeviceRuntimeAccessUseCase
}

type DeviceCatalogUseCase interface {
	ListDevices(ctx context.Context, ownerID string) ([]DeviceView, error)
	ListVisibleDevices(ctx context.Context, ownerID string) ([]DeviceView, error)
	ListDeviceProfiles(ctx context.Context, ownerID string) ([]DeviceProfileView, error)
	ListVisibleDeviceProfiles(ctx context.Context, ownerID string) ([]DeviceProfileView, error)
	GetDevice(ctx context.Context, deviceID string) (DeviceView, error)
	GetDeviceProfile(ctx context.Context, deviceID string) (DeviceProfileView, error)
}

type DeviceProvisioningUseCase interface {
	RegisterDevice(ctx context.Context, input RegisterDeviceInput) (DeviceProfileView, error)
	UpdateDeviceAlias(ctx context.Context, input UpdateDeviceAliasInput) (DeviceProfileView, error)
	DeleteDevice(ctx context.Context, input DeleteDeviceInput) error
	RenewDevice(ctx context.Context, deviceID string) (DeviceProfileView, error)
}

type DeviceRuntimeAccessUseCase interface {
	UpdateDeviceRuntime(ctx context.Context, input UpdateDeviceRuntimeInput) (DeviceProfileView, error)
	DeviceNetworkConfigs(ctx context.Context, deviceID string) ([]NetworkSummaryView, error)
	DeviceMQTTCredential(ctx context.Context, deviceID string) (*mqttkit.Credential, error)
	DeviceMQTTProfile(ctx context.Context, deviceID string) (DeviceMQTTProfileView, error)
}
