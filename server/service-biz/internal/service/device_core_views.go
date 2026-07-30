package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func deviceView(item model.Device) DeviceView {
	return DeviceView{
		DeviceID:      item.DeviceID,
		OwnerID:       item.OwnerID,
		Name:          item.Name,
		Platform:      item.Platform,
		Alias:         item.Alias,
		OSName:        item.OSName,
		OSVersion:     item.OSVersion,
		PublicKey:     item.PublicKey,
		DeviceVersion: item.DeviceVersion,
		CountryCode:   item.CountryCode,
		RXBytesTotal:  item.RXBytesTotal,
		TXBytesTotal:  item.TXBytesTotal,
		Status:        item.Status,
		LastSeenAt:    item.LastSeenAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func buildDeviceProfile(ctx context.Context, users repository.UserRepository, networks repository.NetworkRepository, device model.Device) (DeviceProfileView, error) {
	globalIP := deviceGlobalIP(device)
	view := DeviceProfileView{
		Device:           deviceView(device),
		NetworkEnabled:   device.Status == "active",
		CurrentVirtualIP: globalIP,
		VirtualIP:        globalIP,
		GlobalIP:         globalIP,
		GlobalName:       networkGlobalName(device.DeviceID, device.Alias, device.Name),
	}
	if device.OwnerID != "" {
		if owner, ok, err := users.GetUser(ctx, device.OwnerID); err != nil {
			return DeviceProfileView{}, err
		} else if ok {
			view.OwnerEmail = owner.Email
		}
	}
	items, err := activeDeviceNetworks(ctx, networks, device.DeviceID)
	if err != nil {
		return DeviceProfileView{}, err
	}
	if len(items) == 0 {
		return view, nil
	}
	activeNetwork := items[0]
	view.ActiveNetworkID = activeNetwork.NetworkID
	view.MembershipStatus = "active"
	return view, nil
}

func activeDeviceNetworks(ctx context.Context, networks repository.NetworkRepository, deviceID string) ([]model.Network, error) {
	items, err := networks.ListNetworksByDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	active := make([]model.Network, 0, len(items))
	for _, network := range items {
		member, ok, err := networks.GetNetworkDevice(ctx, network.NetworkID, deviceID)
		if err != nil {
			return nil, err
		}
		if ok && networkMemberActive(member) {
			active = append(active, network)
		}
	}
	return active, nil
}
