package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type NetworkCoreRepository interface {
	GetNetwork(ctx context.Context, networkID string) (model.Network, bool, error)
	ListNetworksByOwner(ctx context.Context, ownerID string) ([]model.Network, error)
	ListNetworksByDevice(ctx context.Context, deviceID string) ([]model.Network, error)
	SaveNetwork(ctx context.Context, network model.Network) error
	DeleteNetwork(ctx context.Context, networkID string) error
	GetNetworkVersion(ctx context.Context, networkID string) (model.NetworkConfigVersion, bool, error)
	SaveNetworkVersion(ctx context.Context, item model.NetworkConfigVersion) error
	ListNetworkDevices(ctx context.Context, networkID string) ([]model.NetworkDevice, error)
	GetNetworkDevice(ctx context.Context, networkID, deviceID string) (model.NetworkDevice, bool, error)
	SaveNetworkDevice(ctx context.Context, item model.NetworkDevice) error
	DeleteNetworkDevice(ctx context.Context, networkID, deviceID string) error
	ListDeviceInvitesByUser(ctx context.Context, userID string) ([]model.DeviceInvite, error)
	ListDeviceInvitesByNetwork(ctx context.Context, networkID string) ([]model.DeviceInvite, error)
	GetDeviceInvite(ctx context.Context, inviteID string) (model.DeviceInvite, bool, error)
	GetDeviceInviteByCode(ctx context.Context, inviteCode string) (model.DeviceInvite, bool, error)
	SaveDeviceInvite(ctx context.Context, invite model.DeviceInvite) error
}
