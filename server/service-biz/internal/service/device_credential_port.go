package service

import "context"

type DeviceCredentialUseCase interface {
	CreateDeviceCredential(ctx context.Context, input CreateDeviceCredentialInput) (CreatedDeviceCredentialView, error)
	ListDeviceCredentials(ctx context.Context, deviceID string) ([]DeviceCredentialView, error)
	RevokeDeviceCredential(ctx context.Context, credentialID string) (DeviceCredentialView, error)
	ExchangeDeviceCredential(ctx context.Context, input ExchangeDeviceCredentialInput) (DeviceSessionBoundView, error)
}
