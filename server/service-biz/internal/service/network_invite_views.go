package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func buildDeviceInviteView(ctx context.Context, users repository.UserRepository, devices repository.DeviceRepository, item model.DeviceInvite) (DeviceInviteView, error) {
	view := DeviceInviteView{
		InviteID:      item.InviteID,
		InviteCode:    item.InviteCode,
		InviterUserID: item.InviterUserID,
		NetworkID:     item.NetworkID,
		DeviceID:      item.DeviceID,
		UserID:        item.UserID,
		Status:        item.Status,
		CreatedAt:     item.CreatedAt,
		ExpiresAt:     item.ExpiresAt,
		AcceptedAt:    item.AcceptedAt,
	}

	if item.InviterUserID != "" {
		if inviter, ok, err := users.GetUser(ctx, item.InviterUserID); err != nil {
			return DeviceInviteView{}, err
		} else if ok {
			view.InviterEmail = inviter.Email
		}
	}

	if item.Status == "accepted" {
		view.AcceptedDeviceID = item.DeviceID
		view.AcceptedUserID = item.UserID
	}

	if item.DeviceID != "" && view.AcceptedDeviceID == "" {
		if device, ok, err := devices.GetDevice(ctx, item.DeviceID); err != nil {
			return DeviceInviteView{}, err
		} else if ok {
			view.AcceptedDeviceID = device.DeviceID
		}
	}

	return view, nil
}

func buildNetworkDeviceView(ctx context.Context, devices repository.DeviceRepository, item model.NetworkDevice) (NetworkDeviceView, error) {
	view := NetworkDeviceView{
		NetworkID: item.NetworkID,
		DeviceID:  item.DeviceID,
		Enabled:   item.Enabled,
		Status:    item.Status,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
	if item.DeviceID == "" {
		return view, nil
	}
	device, ok, err := devices.GetDevice(ctx, item.DeviceID)
	if err != nil {
		return NetworkDeviceView{}, err
	}
	if !ok {
		return view, nil
	}
	view.OwnerUserID = device.OwnerID
	view.Alias = device.Alias
	return view, nil
}
