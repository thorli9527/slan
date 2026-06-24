package service

import "context"

type NetworkInviteUseCase interface {
	ListDeviceInvites(ctx context.Context, userID, networkID string) ([]DeviceInviteView, error)
	CreateDeviceInvite(ctx context.Context, input CreateDeviceInviteInput) (DeviceInviteView, error)
	AcceptDeviceInvite(ctx context.Context, input AcceptDeviceInviteInput) (DeviceInviteView, error)
	ListNetworkDevices(ctx context.Context, networkID string) ([]NetworkDeviceView, error)
	AddNetworkDevice(ctx context.Context, input AddNetworkDeviceInput) (NetworkDeviceView, error)
	UpdateNetworkDevice(ctx context.Context, input UpdateNetworkDeviceInput) (NetworkDeviceView, error)
	RemoveNetworkDevice(ctx context.Context, input RemoveNetworkDeviceInput) error
}
