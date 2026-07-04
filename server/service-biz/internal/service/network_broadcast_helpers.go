package service

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func (s DeviceRuntimeAccessService) publishRuntimeMembershipUpdate(
	ctx context.Context,
	device model.Device,
	networkID string,
	member model.NetworkDevice,
	now time.Time,
) error {
	if s.Broadcaster == nil || strings.TrimSpace(networkID) == "" {
		return nil
	}
	network, ok, err := s.Networks.GetNetwork(ctx, networkID)
	if err != nil || !ok {
		return err
	}
	members, err := s.Networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return err
	}
	if err := s.Broadcaster.PublishNetworkMemberStateChanged(ctx, networkBroadcastMemberState{
		Network:   network,
		Device:    device,
		Member:    member,
		Members:   members,
		Now:       now,
		Online:    networkMemberOnline(member) && strings.TrimSpace(device.Status) == "active",
		PrefixLen: networkPrefixLen(network.CIDR),
	}); err != nil {
		return err
	}
	return publishDevicePresenceChanged(ctx, s.Broadcaster, now, networkID, device.DeviceID, member)
}

func networkPrefixLen(cidr string) int {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return 0
	}
	return prefix.Bits()
}
