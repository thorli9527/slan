package service

import "context"

type DeviceSessionUseCase interface {
	BindDeviceSession(ctx context.Context, input BindDeviceSessionInput) (DeviceSessionBoundView, error)
	RenewDeviceSession(ctx context.Context, accessToken string, input RenewDeviceSessionInput) (DeviceSessionBoundView, error)
	AuthenticateDeviceSession(ctx context.Context, accessToken string) (DeviceSessionView, error)
}
