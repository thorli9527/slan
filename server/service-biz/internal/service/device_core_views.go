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
	view := DeviceProfileView{Device: deviceView(device)}
	if device.OwnerID != "" {
		if owner, ok, err := users.GetUser(ctx, device.OwnerID); err != nil {
			return DeviceProfileView{}, err
		} else if ok {
			view.OwnerEmail = owner.Email
		}
	}
	items, err := networks.ListNetworksByDevice(ctx, device.DeviceID)
	if err != nil {
		return DeviceProfileView{}, err
	}
	if len(items) == 0 {
		return view, nil
	}
	activeNetwork := items[0]
	view.ActiveNetworkID = activeNetwork.NetworkID
	view.MembershipStatus = "active"

	deviceIDs := []string{device.DeviceID}
	deviceIDSet := map[string]struct{}{device.DeviceID: {}}
	members, err := networks.ListNetworkDevices(ctx, activeNetwork.NetworkID)
	if err != nil {
		return DeviceProfileView{}, err
	}
	for _, member := range members {
		if !member.Enabled || member.Status != "active" || member.DeviceID == "" {
			continue
		}
		if _, ok := deviceIDSet[member.DeviceID]; ok {
			continue
		}
		deviceIDSet[member.DeviceID] = struct{}{}
		deviceIDs = append(deviceIDs, member.DeviceID)
	}
	globalIPs, _ := assignedNetworkIPMap(activeNetwork.CIDR, deviceIDs)
	globalIP := globalIPs[device.DeviceID]
	view.CurrentVirtualIP = globalIP
	view.VirtualIP = globalIP
	view.GlobalIP = globalIP
	view.GlobalName = networkGlobalName(device.DeviceID, device.Alias, device.Name)
	return view, nil
}
