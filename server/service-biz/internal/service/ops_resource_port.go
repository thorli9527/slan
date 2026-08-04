package service

import "context"

type OpsResourceUseCase interface {
	ListOpsNetworks(ctx context.Context) ([]OpsNetworkView, error)
	CreateOpsNetwork(ctx context.Context, input OpsNetworkInput) (OpsNetworkView, error)
	UpdateOpsNetwork(ctx context.Context, networkID string, input OpsNetworkInput) (OpsNetworkView, error)
	DeleteOpsNetwork(ctx context.Context, networkID string) error
	ListOpsDeviceGroups(ctx context.Context) (OpsDeviceGroupCollectionView, error)
	CreateOpsDeviceGroup(ctx context.Context, input OpsDeviceGroupInput) (DeviceGroupView, error)
	UpdateOpsDeviceGroup(ctx context.Context, groupID string, input OpsDeviceGroupInput) (DeviceGroupView, error)
	DeleteOpsDeviceGroup(ctx context.Context, groupID string) error
	AddOpsDeviceGroupMember(ctx context.Context, groupID, deviceID string) error
	RemoveOpsDeviceGroupMember(ctx context.Context, groupID, deviceID string) error
	AddOpsNetworkDeviceGroup(ctx context.Context, networkID, groupID string) error
	RemoveOpsNetworkDeviceGroup(ctx context.Context, networkID, groupID string) error
}
