package service

import "context"

type DeviceGroupUseCase interface {
	ListDeviceGroups(ctx context.Context, userID string) (DeviceGroupCollectionView, error)
	ListDeviceGroupMembers(ctx context.Context, userID string) ([]DeviceGroupMemberView, error)
	CreateDeviceGroup(ctx context.Context, input CreateDeviceGroupInput) (DeviceGroupView, error)
	UpdateDeviceGroup(ctx context.Context, input UpdateDeviceGroupInput) (DeviceGroupView, error)
	DeleteDeviceGroup(ctx context.Context, input DeleteDeviceGroupInput) error
	SetDeviceGroups(ctx context.Context, input SetDeviceGroupsInput) error
}
