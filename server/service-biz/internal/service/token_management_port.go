package service

import "context"

type UserTokenManagementUseCase interface {
	ListUserSessions(ctx context.Context, userID string) ([]UserManagedSessionView, error)
	RevokeUserSession(ctx context.Context, input RevokeUserManagedSessionInput) (UserManagedSessionView, error)
}

type DeviceTokenManagementUseCase interface {
	ListDeviceSessions(ctx context.Context, deviceID string) ([]DeviceManagedSessionView, error)
	RevokeDeviceSession(ctx context.Context, input RevokeDeviceManagedSessionInput) (DeviceManagedSessionView, error)
}
