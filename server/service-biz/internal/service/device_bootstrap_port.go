package service

import "context"

type DeviceBootstrapKeyUseCase interface {
	CreateDeviceBootstrapKey(ctx context.Context, input CreateDeviceBootstrapKeyInput) (DeviceBootstrapKeyView, error)
	ListDeviceBootstrapKeys(ctx context.Context, userID string) ([]DeviceBootstrapKeyView, error)
	RevokeDeviceBootstrapKey(ctx context.Context, input RevokeDeviceBootstrapKeyInput) (DeviceBootstrapKeyView, error)
	CleanupExpiredDeviceBootstrapKeys(ctx context.Context) (int64, error)
}

type DeviceBootstrapSessionUseCase interface {
	BootstrapDeviceSession(ctx context.Context, input BootstrapDeviceSessionInput) (DeviceSessionBootstrapView, error)
}

type DeviceBootstrapUseCase interface {
	DeviceBootstrapKeyUseCase
	DeviceBootstrapSessionUseCase
}
