package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func buildManagedDeviceView(
	ctx context.Context,
	networks repository.NetworkRepository,
	device model.Device,
) (OpsManagedDeviceView, error) {
	return managedDeviceView(ctx, networks, device), nil
}

func managedDeviceView(ctx context.Context, networks repository.NetworkRepository, device model.Device) OpsManagedDeviceView {
	view := OpsManagedDeviceView{
		Device:          deviceView(device),
		GlobalIP:        deviceGlobalIP(device),
		GlobalName:      networkGlobalName(device.DeviceID, device.Alias, device.Name),
		HeartbeatOnline: deviceHeartbeatOnlineAt(device, time.Now()),
		NetworkEnabled:  device.Status == "active",
		DeviceEnabled:   device.Status == "active",
	}
	attachedNetworks, err := networks.ListNetworksByDevice(ctx, device.DeviceID)
	if err != nil || len(attachedNetworks) == 0 {
		return view
	}
	view.NetworkCount = len(attachedNetworks)
	return view
}

func deviceHeartbeatOnlineAt(device model.Device, now time.Time) bool {
	if device.Status != "active" || device.LastSeenAt <= 0 {
		return false
	}
	return device.LastSeenAt >= now.Add(-deviceOnlineFreshnessWindow).Unix()
}
