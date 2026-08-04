package service

import "context"

type DeviceGroupUseCase interface {
	ListDeviceGroups(ctx context.Context) (DeviceGroupCollectionView, error)
	ListNetworkDeviceGroups(ctx context.Context, networkID string) (DeviceGroupCollectionView, error)
	AddNetworkDeviceGroup(ctx context.Context, input AddNetworkDeviceGroupInput) (DeviceGroupCollectionView, error)
	RemoveNetworkDeviceGroup(ctx context.Context, input RemoveNetworkDeviceGroupInput) (DeviceGroupCollectionView, error)
	ListDeviceGroupMembers(ctx context.Context) ([]DeviceGroupMemberView, error)
	CreateDeviceGroup(ctx context.Context, input CreateDeviceGroupInput) (DeviceGroupView, error)
	UpdateDeviceGroup(ctx context.Context, input UpdateDeviceGroupInput) (DeviceGroupView, error)
	DeleteDeviceGroup(ctx context.Context, input DeleteDeviceGroupInput) error
	SetDeviceGroups(ctx context.Context, input SetDeviceGroupsInput) error
}
