package service

import "strings"

func normalizeDeviceInviteScope(userID, networkID string) (string, string) {
	return strings.TrimSpace(userID), strings.TrimSpace(networkID)
}

func normalizeCreateDeviceInviteInput(input CreateDeviceInviteInput) CreateDeviceInviteInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.InviterUserID = strings.TrimSpace(input.InviterUserID)
	input.OwnerUserID = strings.TrimSpace(input.OwnerUserID)
	return input
}

func normalizeAcceptDeviceInviteInput(input AcceptDeviceInviteInput) AcceptDeviceInviteInput {
	input.InviteID = strings.TrimSpace(input.InviteID)
	input.InviteCode = strings.TrimSpace(input.InviteCode)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Alias = strings.TrimSpace(input.Alias)
	return input
}

func normalizeRevokeDeviceInviteInput(input RevokeDeviceInviteInput) RevokeDeviceInviteInput {
	input.InviteID = strings.TrimSpace(input.InviteID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}
