package service

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func (s DeviceGroupService) ListDeviceGroups(ctx context.Context) (DeviceGroupCollectionView, error) {
	items, err := s.Devices.ListDeviceGroups(ctx)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	members, err := s.ListDeviceGroupMembers(ctx)
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
	if _, err := requireManagedNetwork(ctx, s.Networks, networkID); err != nil {
		return DeviceGroupCollectionView{}, err
	}
	items, err := s.Devices.ListDeviceGroups(ctx)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	groups, err := s.requireNetworkDeviceGroupRepository()
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	references, err := groups.ListNetworkDeviceGroupReferences(ctx, networkID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	referencedGroupIDs := make(map[string]struct{}, len(references))
	for _, reference := range references {
		referencedGroupIDs[reference.GroupID] = struct{}{}
	}
	assignments, err := s.Devices.ListDeviceGroupAssignments(ctx)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	views := make([]DeviceGroupView, 0, len(referencedGroupIDs))
	for _, item := range items {
		if _, ok := referencedGroupIDs[item.GroupID]; !ok {
			continue
		}
		views = append(views, deviceGroupView(item))
	}
	members := make([]DeviceGroupMemberView, 0)
	for _, assignment := range assignments {
		for _, groupID := range assignment.GroupIDs {
			if _, ok := referencedGroupIDs[groupID]; !ok {
				continue
			}
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

func (s DeviceGroupService) ListDeviceGroupMembers(ctx context.Context) ([]DeviceGroupMemberView, error) {
	assignments, err := s.Devices.ListDeviceGroupAssignments(ctx)
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
	if input.Name == "" {
		return DeviceGroupView{}, ErrInvalidArgument
	}
	now := deviceNow(s.Now).Unix()
	group := model.DeviceGroup{GroupID: newDeviceGroupID(s.Devices), Name: input.Name, Description: input.Description, CreatedAt: now, UpdatedAt: now}
	if err := s.Devices.SaveDeviceGroup(ctx, group); err != nil {
		return DeviceGroupView{}, err
	}
	if err := s.publishDeviceGroupChangesForAllNetworks(ctx, "device_group_created"); err != nil {
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
	if input.Name != "" {
		group.Name = input.Name
	}
	group.Description = input.Description
	group.UpdatedAt = deviceNow(s.Now).Unix()
	if err := s.Devices.SaveDeviceGroup(ctx, group); err != nil {
		return DeviceGroupView{}, err
	}
	if err := s.publishDeviceGroupChangesForAllNetworks(ctx, "device_group_updated"); err != nil {
		return DeviceGroupView{}, err
	}
	return deviceGroupView(group), nil
}

func (s DeviceGroupService) DeleteDeviceGroup(ctx context.Context, input DeleteDeviceGroupInput) error {
	input = normalizeDeleteDeviceGroupInput(input)
	if input.GroupID == "" {
		return ErrInvalidArgument
	}
	_, ok, err := s.Devices.GetDeviceGroup(ctx, input.GroupID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	groups, err := s.requireNetworkDeviceGroupRepository()
	if err != nil {
		return err
	}
	if err := groups.DeleteNetworkDeviceGroupReferencesByGroup(ctx, input.GroupID); err != nil {
		return err
	}
	if err := s.Devices.DeleteDeviceGroup(ctx, input.GroupID); err != nil {
		return err
	}
	return s.syncAllNetworkDeviceGroups(ctx, "device_group_deleted")
}

func (s DeviceGroupService) SetDeviceGroups(ctx context.Context, input SetDeviceGroupsInput) error {
	input = normalizeSetDeviceGroupsInput(input)
	if input.DeviceID == "" {
		return ErrInvalidArgument
	}
	_, err := requireManagedDevice(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return err
	}
	previousGroupIDs, err := s.deviceGroupIDsForDevice(ctx, input.DeviceID)
	if err != nil {
		return err
	}
	for _, groupID := range input.GroupIDs {
		_, ok, err := s.Devices.GetDeviceGroup(ctx, groupID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
	}
	if err := s.Devices.SetDeviceGroups(ctx, model.DeviceGroupAssignment{
		DeviceID:  input.DeviceID,
		GroupIDs:  input.GroupIDs,
		UpdatedAt: deviceNow(s.Now).Unix(),
	}); err != nil {
		return err
	}
	return s.syncDeviceGroupAssignmentNetworks(
		ctx,
		previousGroupIDs,
		input.GroupIDs,
		"device_group_assignment_updated",
	)
}

func (s DeviceGroupService) deviceGroupIDsForDevice(ctx context.Context, deviceID string) ([]string, error) {
	assignments, err := s.Devices.ListDeviceGroupAssignments(ctx)
	if err != nil {
		return nil, err
	}
	for _, assignment := range assignments {
		if strings.TrimSpace(assignment.DeviceID) == deviceID {
			return append([]string(nil), assignment.GroupIDs...), nil
		}
	}
	return nil, nil
}

func (s DeviceGroupService) publishDeviceGroupChangesForAllNetworks(ctx context.Context, reason string) error {
	if s.Networks == nil {
		return nil
	}
	lister, ok := s.Networks.(interface {
		ListNetworks(context.Context) ([]model.Network, error)
	})
	if !ok {
		return ErrNotImplemented
	}
	items, err := lister.ListNetworks(ctx)
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
		if err := publishNetworkSnapshot(ctx, s.Devices, s.Networks, nil, s.EventPublisher, s.Now, networkID, version.Version, version.Reason); err != nil {
			return err
		}
	}
	return nil
}
