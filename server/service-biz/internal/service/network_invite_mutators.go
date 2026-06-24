package service

import (
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func newDeviceInvite(id, code string, createdAt, expiresAt int64, input CreateDeviceInviteInput) model.DeviceInvite {
	return model.DeviceInvite{
		InviteID:      id,
		InviteCode:    strings.ToUpper(code),
		InviterUserID: firstNonEmpty(input.InviterUserID, input.UserID),
		NetworkID:     input.NetworkID,
		DeviceID:      input.DeviceID,
		UserID:        input.UserID,
		Status:        "pending",
		CreatedAt:     createdAt,
		ExpiresAt:     expiresAt,
		AcceptedAt:    0,
	}
}

func acceptStandaloneInvite(invite model.DeviceInvite, userID, deviceID string, acceptedAt int64) model.DeviceInvite {
	invite.Status = "accepted"
	invite.DeviceID = deviceID
	invite.UserID = firstNonEmpty(userID, invite.UserID)
	invite.AcceptedAt = acceptedAt
	return invite
}

func acceptNetworkInvite(invite model.DeviceInvite, userID string, networkDevice model.NetworkDevice, acceptedAt int64) model.DeviceInvite {
	invite.Status = "accepted"
	invite.DeviceID = networkDevice.DeviceID
	invite.UserID = firstNonEmpty(userID, invite.UserID)
	invite.AcceptedAt = acceptedAt
	return invite
}

func newNetworkDeviceMembership(networkID, deviceID string, enabled bool, now int64) model.NetworkDevice {
	return model.NetworkDevice{
		NetworkID: networkID,
		DeviceID:  deviceID,
		Enabled:   enabled,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func applyUpdateNetworkDeviceInput(item model.NetworkDevice, input UpdateNetworkDeviceInput, now int64) model.NetworkDevice {
	if input.Enabled != nil {
		item.Enabled = *input.Enabled
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	item.UpdatedAt = now
	return item
}
