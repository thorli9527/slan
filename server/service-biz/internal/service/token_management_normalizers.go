package service

import "strings"

func normalizeRevokeUserManagedSessionInput(input RevokeUserManagedSessionInput) RevokeUserManagedSessionInput {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.SessionID = strings.TrimSpace(input.SessionID)
	return input
}

func normalizeRevokeDeviceManagedSessionInput(input RevokeDeviceManagedSessionInput) RevokeDeviceManagedSessionInput {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.SessionID = strings.TrimSpace(input.SessionID)
	return input
}
