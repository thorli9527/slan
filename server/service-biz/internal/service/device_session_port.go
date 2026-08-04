package service

import "context"

type DeviceSessionUseCase interface {
	RenewDeviceSession(ctx context.Context, accessToken string, input RenewDeviceSessionInput) (DeviceSessionBoundView, error)
	AuthenticateDeviceSession(ctx context.Context, accessToken string) (DeviceSessionView, error)
}
