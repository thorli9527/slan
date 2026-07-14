package service

import "context"

type NetworkInviteUseCase interface {
	ListDeviceInvites(ctx context.Context, userID, networkID string) ([]DeviceInviteView, error)
	CreateDeviceInvite(ctx context.Context, input CreateDeviceInviteInput) (DeviceInviteView, error)
	AcceptDeviceInvite(ctx context.Context, input AcceptDeviceInviteInput) (DeviceInviteView, error)
	ListNetworkDevices(ctx context.Context, networkID string) ([]NetworkDeviceView, error)
}

type NetworkDeviceQueryUseCase interface {
	ListNetworkDevices(ctx context.Context, networkID string) ([]NetworkDeviceView, error)
}
