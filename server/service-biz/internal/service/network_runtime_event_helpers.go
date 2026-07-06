package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func (s DeviceRuntimeAccessService) publishRuntimeMembershipEvent(
	ctx context.Context,
	device model.Device,
	networkID string,
	member model.NetworkDevice,
	now time.Time,
) error {
	if s.EventPublisher == nil || strings.TrimSpace(networkID) == "" {
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
	_ = network
	_ = members
	_ = device
	return publishDevicePresenceChanged(ctx, s.EventPublisher, now, networkID, device.DeviceID, member)
}
