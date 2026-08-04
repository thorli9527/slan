package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func newNetworkDeviceMembership(networkID, deviceID string, groupIDs []string, now int64) model.NetworkDevice {
	return model.NetworkDevice{
		NetworkID:      networkID,
		DeviceID:       deviceID,
		DeviceGroupIDs: append([]string(nil), groupIDs...),
		Enabled:        true,
		MemberStatus:   model.NetworkMemberStatusActive,
		PresenceStatus: model.DevicePresenceStatusOffline,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (s DeviceGroupService) AddNetworkDeviceGroup(ctx context.Context, input AddNetworkDeviceGroupInput) (DeviceGroupCollectionView, error) {
	input.NetworkID = normalizeNetworkID(input.NetworkID)
	input.GroupID = normalizeDeviceGroupID(input.GroupID)
	if input.NetworkID == "" || input.GroupID == "" {
		return DeviceGroupCollectionView{}, ErrInvalidArgument
	}
	network, err := requireManagedNetwork(ctx, s.Networks, input.NetworkID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	group, ok, err := s.Devices.GetDeviceGroup(ctx, input.GroupID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	if !ok {
		return DeviceGroupCollectionView{}, ErrNotFound
	}
	now := deviceNow(s.Now).Unix()
	groups, err := s.requireNetworkDeviceGroupRepository()
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	if err := groups.SaveNetworkDeviceGroupReference(ctx, model.NetworkDeviceGroupReference{
		NetworkID: network.NetworkID,
		GroupID:   group.GroupID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return DeviceGroupCollectionView{}, err
	}
	if err := s.syncNetworkDeviceGroupMemberships(ctx, network, "network_device_group_added"); err != nil {
		return DeviceGroupCollectionView{}, err
	}
	if err := publishNetworkDeviceGroupChanged(ctx, s.Devices, s.Networks, s.EventPublisher, s.Now, network.NetworkID, group.GroupID, NetworkEventDeviceGroupAdded); err != nil {
		return DeviceGroupCollectionView{}, err
	}
	return s.ListNetworkDeviceGroups(ctx, network.NetworkID)
}

func (s DeviceGroupService) RemoveNetworkDeviceGroup(ctx context.Context, input RemoveNetworkDeviceGroupInput) (DeviceGroupCollectionView, error) {
	input.NetworkID = normalizeNetworkID(input.NetworkID)
	input.GroupID = normalizeDeviceGroupID(input.GroupID)
	if input.NetworkID == "" || input.GroupID == "" {
		return DeviceGroupCollectionView{}, ErrInvalidArgument
	}
	network, err := requireManagedNetwork(ctx, s.Networks, input.NetworkID)
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	groups, err := s.requireNetworkDeviceGroupRepository()
	if err != nil {
		return DeviceGroupCollectionView{}, err
	}
	if err := groups.DeleteNetworkDeviceGroupReference(ctx, network.NetworkID, input.GroupID); err != nil {
		return DeviceGroupCollectionView{}, err
	}
	if err := s.syncNetworkDeviceGroupMemberships(ctx, network, "network_device_group_removed"); err != nil {
		return DeviceGroupCollectionView{}, err
	}
	if err := publishNetworkDeviceGroupChanged(ctx, s.Devices, s.Networks, s.EventPublisher, s.Now, network.NetworkID, input.GroupID, NetworkEventDeviceGroupRemoved); err != nil {
		return DeviceGroupCollectionView{}, err
	}
	return s.ListNetworkDeviceGroups(ctx, network.NetworkID)
}

func (s DeviceGroupService) syncAllNetworkDeviceGroups(ctx context.Context, reason string) error {
	lister, ok := s.Networks.(interface {
		ListNetworks(context.Context) ([]model.Network, error)
	})
	if !ok {
		return ErrNotImplemented
	}
	networks, err := lister.ListNetworks(ctx)
	if err != nil {
		return err
	}
	for _, network := range networks {
		if err := s.syncNetworkDeviceGroupMemberships(ctx, network, reason); err != nil {
			return err
		}
	}
	return nil
}

func (s DeviceGroupService) syncDeviceGroupAssignmentNetworks(
	ctx context.Context,
	previousGroupIDs []string,
	currentGroupIDs []string,
	reason string,
) error {
	impactedGroupIDs := make(map[string]struct{}, len(previousGroupIDs)+len(currentGroupIDs))
	for _, groupID := range append(append([]string(nil), previousGroupIDs...), currentGroupIDs...) {
		groupID = normalizeDeviceGroupID(groupID)
		if groupID != "" {
			impactedGroupIDs[groupID] = struct{}{}
		}
	}
	if len(impactedGroupIDs) == 0 {
		return nil
	}
	lister, ok := s.Networks.(interface {
		ListNetworks(context.Context) ([]model.Network, error)
	})
	if !ok {
		return ErrNotImplemented
	}
	groups, err := s.requireNetworkDeviceGroupRepository()
	if err != nil {
		return err
	}
	networks, err := lister.ListNetworks(ctx)
	if err != nil {
		return err
	}
	for _, network := range networks {
		references, err := groups.ListNetworkDeviceGroupReferences(ctx, network.NetworkID)
		if err != nil {
			return err
		}
		impacted := false
		for _, reference := range references {
			if _, ok := impactedGroupIDs[normalizeDeviceGroupID(reference.GroupID)]; ok {
				impacted = true
				break
			}
		}
		if impacted {
			if err := s.syncNetworkDeviceGroupMemberships(ctx, network, reason); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s DeviceGroupService) syncNetworkDeviceGroupMemberships(ctx context.Context, network model.Network, reason string) error {
	groups, err := s.requireNetworkDeviceGroupRepository()
	if err != nil {
		return err
	}
	references, err := groups.ListNetworkDeviceGroupReferences(ctx, network.NetworkID)
	if err != nil {
		return err
	}
	referenced := make(map[string]struct{}, len(references))
	for _, reference := range references {
		referenced[reference.GroupID] = struct{}{}
	}
	assignments, err := s.Devices.ListDeviceGroupAssignments(ctx)
	if err != nil {
		return err
	}
	desired := make(map[string][]string)
	for _, assignment := range assignments {
		for _, groupID := range assignment.GroupIDs {
			if _, ok := referenced[groupID]; ok {
				desired[assignment.DeviceID] = append(desired[assignment.DeviceID], groupID)
			}
		}
	}
	existing, err := s.Networks.ListNetworkDevices(ctx, network.NetworkID)
	if err != nil {
		return err
	}
	existingByID := make(map[string]model.NetworkDevice, len(existing))
	for _, member := range existing {
		existingByID[member.DeviceID] = member
	}
	now := deviceNow(s.Now).Unix()
	type membershipChange struct {
		action string
		member model.NetworkDevice
	}
	changedDevices := make(map[string]membershipChange)
	for deviceID, groupIDs := range desired {
		device, ok, err := s.Devices.GetDevice(ctx, deviceID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if !managedDeviceVirtualIP(device.VirtualIP) {
			device.VirtualIP, err = allocateDeviceVirtualIP(s.Devices)
			if err != nil {
				return err
			}
			device.UpdatedAt = now
			if err := s.Devices.SaveDevice(ctx, device); err != nil {
				return err
			}
		}
		if existing, ok := existingByID[deviceID]; ok {
			previousGroupIDs := append([]string(nil), existing.DeviceGroupIDs...)
			existing.DeviceGroupIDs = append([]string(nil), groupIDs...)
			existing.Enabled = true
			existing.MemberStatus = model.NetworkMemberStatusActive
			existing.UpdatedAt = now
			if err := s.Networks.SaveNetworkDevice(ctx, existing); err != nil {
				return err
			}
			if !sameDeviceGroupIDs(previousGroupIDs, groupIDs) {
				changedDevices[deviceID] = membershipChange{action: "updated", member: existing}
			}
			continue
		}
		member := newNetworkDeviceMembership(network.NetworkID, deviceID, groupIDs, now)
		if err := s.Networks.SaveNetworkDevice(ctx, member); err != nil {
			return err
		}
		changedDevices[deviceID] = membershipChange{action: "added", member: member}
	}
	for deviceID := range existingByID {
		if _, ok := desired[deviceID]; ok {
			continue
		}
		existing := existingByID[deviceID]
		existing.DeviceGroupIDs = nil
		existing.UpdatedAt = now
		if existing.Direct {
			if err := s.Networks.SaveNetworkDevice(ctx, existing); err != nil {
				return err
			}
			changedDevices[deviceID] = membershipChange{action: "updated", member: existing}
			continue
		}
		if err := s.Networks.DeleteNetworkDevice(ctx, network.NetworkID, deviceID); err != nil {
			return err
		}
		changedDevices[deviceID] = membershipChange{action: "removed", member: existing}
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, network.NetworkID, reason)
	if err != nil {
		return err
	}
	changedDeviceIDs := make([]string, 0, len(changedDevices))
	for deviceID := range changedDevices {
		changedDeviceIDs = append(changedDeviceIDs, deviceID)
	}
	slices.Sort(changedDeviceIDs)
	for _, deviceID := range changedDeviceIDs {
		change := changedDevices[deviceID]
		if err := publishNetworkMemberChanged(ctx, s.EventPublisher, s.Now, network.NetworkID, deviceID, change.action, change.member, version.Version, version.Reason); err != nil {
			return err
		}
	}
	if err := publishACLChanged(ctx, s.Networks, s.EventPublisher, s.Now, network.NetworkID, version.Version, version.Reason); err != nil {
		return err
	}
	if err := publishNetworkSnapshot(ctx, s.Devices, s.Networks, nil, s.EventPublisher, s.Now, network.NetworkID, version.Version, version.Reason); err != nil {
		return err
	}
	for _, deviceID := range changedDeviceIDs {
		operation := changedDevices[deviceID].action
		if operation == "added" {
			operation = "joined"
		} else if operation == "removed" {
			operation = "left"
		} else {
			continue
		}
		if err := s.publishGroupDerivedNetworkMembership(ctx, deviceID, network.NetworkID, operation, version.Version); err != nil {
			return err
		}
	}
	return nil
}

func sameDeviceGroupIDs(left, right []string) bool {
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(left, right)
}

func (s DeviceGroupService) publishGroupDerivedNetworkMembership(ctx context.Context, deviceID, changedNetworkID, operation string, membershipVersion int64) error {
	if s.DevicePublisher == nil {
		return nil
	}
	networks, err := s.Networks.ListNetworksByDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	networkIDs := make([]string, 0, len(networks))
	for _, network := range networks {
		networkIDs = append(networkIDs, network.NetworkID)
	}
	now := deviceNow(s.Now)
	return s.DevicePublisher.PublishDeviceControl(ctx, deviceID, DeviceControlEnvelope{
		Type:      "device_network_membership_changed",
		MessageID: fmt.Sprintf("devgroupnet%d%s", now.UnixMilli(), deviceID),
		Payload: map[string]any{
			"deviceId":          deviceID,
			"changedNetworkId":  changedNetworkID,
			"networkIds":        networkIDs,
			"membershipVersion": membershipVersion,
			"operation":         operation,
			"source":            "device_group_reference",
		},
	})
}

func (s DeviceGroupService) requireNetworkDeviceGroupRepository() (repository.NetworkDeviceGroupRepository, error) {
	if s.NetworkGroups != nil {
		return s.NetworkGroups, nil
	}
	if groups, ok := s.Networks.(repository.NetworkDeviceGroupRepository); ok {
		return groups, nil
	}
	return nil, ErrNotImplemented
}
