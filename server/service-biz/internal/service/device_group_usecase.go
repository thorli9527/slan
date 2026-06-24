package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s DeviceGroupService) ListDeviceGroups(ctx context.Context, userID string) (DeviceGroupCollectionView, error) {
	userID = normalizeDeviceBootstrapUserID(userID)
	items, err := s.Devices.ListDeviceGroups(ctx, userID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	members, err := s.ListDeviceGroupMembers(ctx, userID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	views := make([]DeviceGroupView, 0, len(items))
	for _, item := range items {
		views = append(views, deviceGroupView(item))
	}
	return DeviceGroupCollectionView{
		Items:   views,
		Members: members,
	}, nil
}

func (s DeviceGroupService) ListDeviceGroupMembers(ctx context.Context, userID string) ([]DeviceGroupMemberView, error) {
	assignments, err := s.Devices.ListDeviceGroupAssignments(ctx, normalizeDeviceBootstrapUserID(userID))
	if err != nil {
		return nil, err
	}
	items := make([]DeviceGroupMemberView, 0)
	for _, assignment := range assignments {
		for _, groupID := range assignment.GroupIDs {
			items = append(items, DeviceGroupMemberView{
				GroupID:  groupID,
				DeviceID: assignment.DeviceID,
				AddedAt:  assignment.UpdatedAt,
			})
		}
	}
	return items, nil
}

func (s DeviceGroupService) CreateDeviceGroup(ctx context.Context, input CreateDeviceGroupInput) (DeviceGroupView, error) {
	input = normalizeCreateDeviceGroupInput(input)
	if input.UserID == "" || input.Name == "" {
		return DeviceGroupView{}, ErrInvalidArgument
	}
	if input.ActorUserID != "" && input.ActorUserID != input.UserID {
		return DeviceGroupView{}, ErrUnauthorized
	}
	if _, ok, err := s.Users.GetUser(ctx, input.UserID); err != nil {
		return DeviceGroupView{}, err
	} else if !ok {
		return DeviceGroupView{}, ErrNotFound
	}
	now := deviceNow(s.Now).Unix()
	group := model.DeviceGroup{GroupID: newDeviceGroupID(s.Devices), UserID: input.UserID, Name: input.Name, Description: input.Description, CreatedAt: now, UpdatedAt: now}
	if err := s.Devices.SaveDeviceGroup(ctx, group); err != nil {
		return DeviceGroupView{}, err
	}
	return deviceGroupView(group), nil
}

func (s DeviceGroupService) UpdateDeviceGroup(ctx context.Context, input UpdateDeviceGroupInput) (DeviceGroupView, error) {
	input = normalizeUpdateDeviceGroupInput(input)
	if input.GroupID == "" {
		return DeviceGroupView{}, ErrInvalidArgument
	}
	group, ok, err := s.Devices.GetDeviceGroup(ctx, input.GroupID)
	if err != nil {
		return DeviceGroupView{}, err
	}
	if !ok {
		return DeviceGroupView{}, ErrNotFound
	}
	if input.ActorUserID != "" && input.ActorUserID != group.UserID {
		return DeviceGroupView{}, ErrUnauthorized
	}
	if input.Name != "" {
		group.Name = input.Name
	}
	group.Description = input.Description
	group.UpdatedAt = deviceNow(s.Now).Unix()
	if err := s.Devices.SaveDeviceGroup(ctx, group); err != nil {
		return DeviceGroupView{}, err
	}
	return deviceGroupView(group), nil
}

func (s DeviceGroupService) DeleteDeviceGroup(ctx context.Context, input DeleteDeviceGroupInput) error {
	input = normalizeDeleteDeviceGroupInput(input)
	if input.GroupID == "" {
		return ErrInvalidArgument
	}
	group, ok, err := s.Devices.GetDeviceGroup(ctx, input.GroupID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	if input.ActorUserID != "" && input.ActorUserID != group.UserID {
		return ErrUnauthorized
	}
	return s.Devices.DeleteDeviceGroup(ctx, input.GroupID)
}

func (s DeviceGroupService) SetDeviceGroups(ctx context.Context, input SetDeviceGroupsInput) error {
	input = normalizeSetDeviceGroupsInput(input)
	if input.UserID == "" || input.DeviceID == "" {
		return ErrInvalidArgument
	}
	if input.ActorUserID != "" && input.ActorUserID != input.UserID {
		return ErrUnauthorized
	}
	device, err := requireOwnedManagedDevice(ctx, s.Users, s.Devices, input.ActorUserID, input.DeviceID)
	if err != nil {
		return err
	}
	if device.OwnerID != input.UserID {
		return ErrUnauthorized
	}
	for _, groupID := range input.GroupIDs {
		group, ok, err := s.Devices.GetDeviceGroup(ctx, normalizeDeviceGroupID(groupID))
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if group.UserID != input.UserID {
			return ErrUnauthorized
		}
	}
	return s.Devices.SetDeviceGroups(ctx, model.DeviceGroupAssignment{
		UserID:    input.UserID,
		DeviceID:  input.DeviceID,
		GroupIDs:  input.GroupIDs,
		UpdatedAt: deviceNow(s.Now).Unix(),
	})
}
