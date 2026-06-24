package service

import "strings"

func normalizeCreateDeviceBootstrapKeyInput(input CreateDeviceBootstrapKeyInput) CreateDeviceBootstrapKeyInput {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.Name = strings.TrimSpace(input.Name)
	return input
}

func normalizeRevokeDeviceBootstrapKeyInput(input RevokeDeviceBootstrapKeyInput) RevokeDeviceBootstrapKeyInput {
	input.KeyID = strings.TrimSpace(input.KeyID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}

func normalizeBootstrapDeviceSessionInput(input BootstrapDeviceSessionInput) BootstrapDeviceSessionInput {
	input.SessionKey = strings.TrimSpace(input.SessionKey)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.OwnerID = strings.TrimSpace(input.OwnerID)
	input.Name = strings.TrimSpace(input.Name)
	input.Platform = strings.TrimSpace(input.Platform)
	input.Alias = strings.TrimSpace(input.Alias)
	input.OSName = strings.TrimSpace(input.OSName)
	input.OSVersion = strings.TrimSpace(input.OSVersion)
	input.PublicKey = strings.TrimSpace(input.PublicKey)
	input.DeviceVersion = strings.TrimSpace(input.DeviceVersion)
	input.CountryCode = strings.TrimSpace(input.CountryCode)
	return input
}

func normalizeDeviceBootstrapUserID(userID string) string {
	return strings.TrimSpace(userID)
}
