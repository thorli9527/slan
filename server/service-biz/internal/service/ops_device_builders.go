package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func buildManagedDeviceViews(
	ctx context.Context,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	users []model.User,
) ([]OpsManagedDeviceView, error) {
	items := make([]OpsManagedDeviceView, 0)
	for _, user := range users {
		userDevices, err := devices.ListDevicesByOwner(ctx, user.UserID)
		if err != nil {
			return nil, err
		}
		for _, device := range userDevices {
			items = append(items, managedDeviceView(ctx, networks, device, user.Email))
		}
	}
	return items, nil
}

func buildManagedDeviceView(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	device model.Device,
) (OpsManagedDeviceView, error) {
	ownerEmail := ""
	if device.OwnerID != "" {
		owner, ok, err := users.GetUser(ctx, device.OwnerID)
		if err != nil {
			return OpsManagedDeviceView{}, err
		}
		if ok {
			ownerEmail = owner.Email
		}
	}
	return managedDeviceView(ctx, networks, device, ownerEmail), nil
}

func managedDeviceView(ctx context.Context, networks repository.NetworkRepository, device model.Device, ownerEmail string) OpsManagedDeviceView {
	view := OpsManagedDeviceView{
		Device:          deviceView(device),
		OwnerEmail:      ownerEmail,
		HeartbeatOnline: device.Status == "active",
		NetworkEnabled:  device.Status == "active",
		DeviceEnabled:   device.Status == "active",
	}
	attachedNetworks, err := networks.ListNetworksByDevice(ctx, device.DeviceID)
	if err != nil || len(attachedNetworks) == 0 {
		return view
	}
	view.NetworkCount = len(attachedNetworks)
	view.GlobalIP = managedDeviceGlobalIP(ctx, networks, device.DeviceID, attachedNetworks[0])
	view.GlobalName = networkGlobalName(device.DeviceID, device.Alias, device.Name)
	return view
}
