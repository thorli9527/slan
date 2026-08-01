package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func (s DeviceGroupService) AddNetworkDeviceGroup(ctx context.Context, input AddNetworkDeviceGroupInput) (DeviceGroupCollectionView, error) {
	input.NetworkID = normalizeNetworkID(input.NetworkID)
	input.GroupID = normalizeDeviceGroupID(input.GroupID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	if input.NetworkID == "" || input.GroupID == "" || input.ActorUserID == "" {
		return DeviceGroupCollectionView{}, ErrInvalidArgument
	}
	network, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID)
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
	if group.UserID != network.OwnerID {
		return DeviceGroupCollectionView{}, ErrForbidden
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
	return s.ListNetworkDeviceGroups(ctx, network.NetworkID)
}

func (s DeviceGroupService) RemoveNetworkDeviceGroup(ctx context.Context, input RemoveNetworkDeviceGroupInput) (DeviceGroupCollectionView, error) {
	input.NetworkID = normalizeNetworkID(input.NetworkID)
	input.GroupID = normalizeDeviceGroupID(input.GroupID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	if input.NetworkID == "" || input.GroupID == "" || input.ActorUserID == "" {
		return DeviceGroupCollectionView{}, ErrInvalidArgument
	}
	network, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID)
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
	return s.ListNetworkDeviceGroups(ctx, network.NetworkID)
}

func (s DeviceGroupService) syncOwnerNetworkDeviceGroups(ctx context.Context, ownerUserID, reason string) error {
	networks, err := s.Networks.ListNetworksByOwner(ctx, strings.TrimSpace(ownerUserID))
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
	assignments, err := s.Devices.ListDeviceGroupAssignments(ctx, network.OwnerID)
	if err != nil {
		return err
	}
	desired := make(map[string]struct{})
	for _, assignment := range assignments {
		for _, groupID := range assignment.GroupIDs {
			if _, ok := referenced[groupID]; ok {
				desired[assignment.DeviceID] = struct{}{}
				break
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
	changedDevices := make(map[string]string)
	for deviceID := range desired {
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
		if _, ok := existingByID[deviceID]; ok {
			continue
		}
		member := newNetworkDeviceMembership(network.NetworkID, deviceID, true, now)
		member.MembershipSource = model.NetworkMembershipSourceDeviceGroup
		if err := s.Networks.SaveNetworkDevice(ctx, member); err != nil {
			return err
		}
		changedDevices[deviceID] = "joined"
	}
	for deviceID, member := range existingByID {
		if _, ok := desired[deviceID]; ok {
			continue
		}
		if member.MembershipSource == model.NetworkMembershipSourceDirect || member.MembershipSource == model.NetworkMembershipSourceExcluded {
			continue
		}
		if err := s.Networks.DeleteNetworkDevice(ctx, network.NetworkID, deviceID); err != nil {
			return err
		}
		changedDevices[deviceID] = "left"
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, network.NetworkID, reason)
	if err != nil {
		return err
	}
	if err := publishACLChanged(ctx, s.Networks, s.EventPublisher, s.Now, network.NetworkID, version.Version, version.Reason); err != nil {
		return err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, nil, s.EventPublisher, s.Now, network.NetworkID, version.Version, version.Reason); err != nil {
		return err
	}
	for deviceID, operation := range changedDevices {
		if err := s.publishGroupDerivedNetworkMembership(ctx, deviceID, network.NetworkID, operation, version.Version); err != nil {
			return err
		}
	}
	return nil
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
