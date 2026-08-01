package service

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func (s NetworkInviteService) ListNetworkDevices(ctx context.Context, networkID string) ([]NetworkDeviceView, error) {
	networkID = normalizeNetworkID(networkID)
	if networkID == "" {
		return []NetworkDeviceView{}, nil
	}
	if _, err := requireManagedNetwork(ctx, s.Networks, networkID); err != nil {
		return nil, err
	}
	items, err := s.Networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return nil, err
	}
	active := make([]model.NetworkDevice, 0, len(items))
	for _, item := range items {
		if networkMemberActive(item) {
			active = append(active, item)
		}
	}
	return buildNetworkDeviceViews(ctx, s.Devices, active)
}

func (s NetworkInviteService) AddNetworkDevice(ctx context.Context, input AddNetworkDeviceInput) (NetworkDeviceView, error) {
	input.NetworkID = normalizeNetworkID(input.NetworkID)
	input.DeviceID = normalizeDeviceID(input.DeviceID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	network, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID)
	if err != nil {
		return NetworkDeviceView{}, err
	}
	device, err := requireManagedDevice(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return NetworkDeviceView{}, err
	}
	if device.OwnerID != network.OwnerID {
		return NetworkDeviceView{}, ErrForbidden
	}
	if existing, ok, getErr := s.Networks.GetNetworkDevice(ctx, input.NetworkID, input.DeviceID); getErr != nil {
		return NetworkDeviceView{}, getErr
	} else if ok && networkMemberActive(existing) {
		return buildNetworkDeviceView(ctx, s.Devices, existing)
	}
	now := networkNow(s.Now)
	member := newNetworkDeviceMembership(input.NetworkID, input.DeviceID, true, now.Unix())
	if err := s.Networks.SaveNetworkDevice(ctx, member); err != nil {
		return NetworkDeviceView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, input.NetworkID, "ops_network_member_added")
	if err != nil {
		return NetworkDeviceView{}, err
	}
	if err := publishNetworkMemberChanged(ctx, s.EventPublisher, s.Now, input.NetworkID, input.DeviceID, "added", member, version.Version, version.Reason); err != nil {
		return NetworkDeviceView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, nil, s.EventPublisher, s.Now, input.NetworkID, version.Version, version.Reason); err != nil {
		return NetworkDeviceView{}, err
	}
	if err := s.publishDirectNetworkMembership(ctx, input.DeviceID, input.NetworkID, "joined", version.Version); err != nil {
		return NetworkDeviceView{}, err
	}
	return buildNetworkDeviceView(ctx, s.Devices, member)
}

func (s NetworkInviteService) RemoveNetworkDevice(ctx context.Context, input RemoveNetworkDeviceInput) error {
	input.NetworkID = normalizeNetworkID(input.NetworkID)
	input.DeviceID = normalizeDeviceID(input.DeviceID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID); err != nil {
		return err
	}
	member, ok, err := s.Networks.GetNetworkDevice(ctx, input.NetworkID, input.DeviceID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	member.Enabled = false
	member.MemberStatus = model.NetworkMemberStatusRemoved
	member.MembershipSource = model.NetworkMembershipSourceExcluded
	member.UpdatedAt = networkNow(s.Now).Unix()
	if err := s.Networks.SaveNetworkDevice(ctx, member); err != nil {
		return err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, input.NetworkID, "ops_network_member_removed")
	if err != nil {
		return err
	}
	if err := publishNetworkMemberChanged(ctx, s.EventPublisher, s.Now, input.NetworkID, input.DeviceID, "removed", member, version.Version, version.Reason); err != nil {
		return err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, nil, s.EventPublisher, s.Now, input.NetworkID, version.Version, version.Reason); err != nil {
		return err
	}
	return s.publishDirectNetworkMembership(ctx, input.DeviceID, input.NetworkID, "left", version.Version)
}

func (s NetworkInviteService) publishDirectNetworkMembership(ctx context.Context, deviceID, changedNetworkID, operation string, version int64) error {
	return publishDirectNetworkMembership(ctx, s.Networks, s.DevicePublisher, s.Now, deviceID, changedNetworkID, operation, version)
}
