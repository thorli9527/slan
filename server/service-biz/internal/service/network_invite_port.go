package service

import "context"

type NetworkInviteUseCase interface {
	ListDeviceInvites(ctx context.Context, userID, networkID string) ([]DeviceInviteView, error)
	CreateDeviceInvite(ctx context.Context, input CreateDeviceInviteInput) (DeviceInviteView, error)
	AcceptDeviceInvite(ctx context.Context, input AcceptDeviceInviteInput) (DeviceInviteView, error)
	RevokeDeviceInvite(ctx context.Context, input RevokeDeviceInviteInput) (DeviceInviteView, error)
	ListNetworkDevices(ctx context.Context, networkID string) ([]NetworkDeviceView, error)
	AddNetworkDevice(ctx context.Context, input AddNetworkDeviceInput) (NetworkDeviceView, error)
	RemoveNetworkDevice(ctx context.Context, input RemoveNetworkDeviceInput) error
}

type AddNetworkDeviceInput struct {
	NetworkID   string `json:"networkId"`
	DeviceID    string `json:"deviceId"`
	ActorUserID string `json:"actorUserId"`
}

type RemoveNetworkDeviceInput struct {
	NetworkID   string `json:"networkId"`
	DeviceID    string `json:"deviceId"`
	ActorUserID string `json:"actorUserId"`
}

type NetworkDeviceQueryUseCase interface {
	ListNetworkDevices(ctx context.Context, networkID string) ([]NetworkDeviceView, error)
}
