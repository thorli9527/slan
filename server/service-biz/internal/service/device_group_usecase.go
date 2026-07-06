package service

import (
	"context"
	"strings"

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

func (s DeviceGroupService) ListNetworkDeviceGroups(ctx context.Context, networkID string) (DeviceGroupCollectionView, error) {
	networkID = normalizeNetworkID(networkID)
	if networkID == "" {
		return DeviceGroupCollectionView{}, ErrInvalidArgument
	}
	network, err := requireManagedNetwork(ctx, s.Networks, networkID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	userID := normalizeDeviceBootstrapUserID(network.OwnerID)
	items, err := s.Devices.ListDeviceGroups(ctx, userID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	assignments, err := s.Devices.ListDeviceGroupAssignments(ctx, userID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	memberships, err := s.Networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	allowedDeviceIDs := make(map[string]struct{}, len(memberships))
	for _, membership := range memberships {
		if membership.DeviceID == "" {
			continue
		}
		allowedDeviceIDs[membership.DeviceID] = struct{}{}
	}
	views := make([]DeviceGroupView, 0, len(items))
	for _, item := range items {
		views = append(views, deviceGroupView(item))
	}
	members := make([]DeviceGroupMemberView, 0)
	for _, assignment := range assignments {
		if _, ok := allowedDeviceIDs[assignment.DeviceID]; !ok {
			continue
		}
		for _, groupID := range assignment.GroupIDs {
			members = append(members, DeviceGroupMemberView{
				GroupID:  groupID,
				DeviceID: assignment.DeviceID,
				AddedAt:  assignment.UpdatedAt,
			})
		}
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
	if err := s.publishDeviceGroupChangesForOwnerNetworks(ctx, input.UserID, "device_group_created"); err != nil {
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
	if err := s.publishDeviceGroupChangesForOwnerNetworks(ctx, group.UserID, "device_group_updated"); err != nil {
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
	if err := s.Devices.DeleteDeviceGroup(ctx, input.GroupID); err != nil {
		return err
	}
	return s.publishDeviceGroupChangesForOwnerNetworks(ctx, group.UserID, "device_group_deleted")
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
	if err := s.Devices.SetDeviceGroups(ctx, model.DeviceGroupAssignment{
		UserID:    input.UserID,
		DeviceID:  input.DeviceID,
		GroupIDs:  input.GroupIDs,
		UpdatedAt: deviceNow(s.Now).Unix(),
	}); err != nil {
		return err
	}
	return s.publishDeviceGroupChangesForDeviceNetworks(ctx, input.DeviceID, "device_group_assignment_updated")
}

func (s DeviceGroupService) publishDeviceGroupChangesForOwnerNetworks(ctx context.Context, ownerUserID, reason string) error {
	if s.Networks == nil {
		return nil
	}
	items, err := s.Networks.ListNetworksByOwner(ctx, strings.TrimSpace(ownerUserID))
	if err != nil {
		return err
	}
	networkIDs := make([]string, 0, len(items))
	for _, item := range items {
		networkIDs = append(networkIDs, item.NetworkID)
	}
	return s.publishDeviceGroupChangesForNetworks(ctx, networkIDs, reason)
}

func (s DeviceGroupService) publishDeviceGroupChangesForDeviceNetworks(ctx context.Context, deviceID, reason string) error {
	if s.Networks == nil {
		return nil
	}
	items, err := s.Networks.ListNetworksByDevice(ctx, strings.TrimSpace(deviceID))
	if err != nil {
		return err
	}
	networkIDs := make([]string, 0, len(items))
	for _, item := range items {
		networkIDs = append(networkIDs, item.NetworkID)
	}
	return s.publishDeviceGroupChangesForNetworks(ctx, networkIDs, reason)
}

func (s DeviceGroupService) publishDeviceGroupChangesForNetworks(ctx context.Context, networkIDs []string, reason string) error {
	seen := make(map[string]struct{}, len(networkIDs))
	for _, networkID := range networkIDs {
		networkID = strings.TrimSpace(networkID)
		if networkID == "" {
			continue
		}
		if _, ok := seen[networkID]; ok {
			continue
		}
		seen[networkID] = struct{}{}
		version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, networkID, reason)
		if err != nil {
			return err
		}
		if err := publishACLChanged(ctx, s.Networks, s.EventPublisher, s.Now, networkID, version.Version, version.Reason); err != nil {
			return err
		}
		if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, nil, s.EventPublisher, s.Now, networkID, version.Version, version.Reason); err != nil {
			return err
		}
	}
	return nil
}
