package service

import "context"

type OpsManagedDeviceUseCase interface {
	ListDevices(ctx context.Context) ([]OpsManagedDeviceView, error)
	CreateDevice(ctx context.Context, input CreateOpsDeviceInput) (OpsManagedDeviceView, error)
	UpdateDevice(ctx context.Context, input UpdateDeviceInput) (OpsManagedDeviceView, error)
	DeleteDevice(ctx context.Context, deviceID string) error
}
