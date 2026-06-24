package service

import "strings"

func normalizeDeviceOwnerID(ownerID string) string {
	return strings.TrimSpace(ownerID)
}

func normalizeRegisterDeviceInput(input RegisterDeviceInput) RegisterDeviceInput {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.OwnerID = strings.TrimSpace(input.OwnerID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
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

func normalizeUpdateDeviceAliasInput(input UpdateDeviceAliasInput) UpdateDeviceAliasInput {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Alias = strings.TrimSpace(input.Alias)
	return input
}

func normalizeUpdateDeviceRuntimeInput(input UpdateDeviceRuntimeInput) UpdateDeviceRuntimeInput {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.Platform = strings.TrimSpace(input.Platform)
	input.Status = strings.TrimSpace(input.Status)
	input.DeviceVersion = strings.TrimSpace(input.DeviceVersion)
	input.NATType = strings.TrimSpace(input.NATType)
	input.ActivePath = strings.TrimSpace(input.ActivePath)
	input.RelayTransport = strings.TrimSpace(input.RelayTransport)
	input.RelayEndpoint = strings.TrimSpace(input.RelayEndpoint)
	input.DerpNodeID = strings.TrimSpace(input.DerpNodeID)
	input.PeerNodeID = strings.TrimSpace(input.PeerNodeID)
	input.TicketExpiresAt = strings.TrimSpace(input.TicketExpiresAt)
	input.LastPathChange = strings.TrimSpace(input.LastPathChange)
	if input.PathScore < 0 {
		input.PathScore = 0
	}
	if input.ObservedRttMs < 0 {
		input.ObservedRttMs = 0
	}
	if input.PacketLossPpm < 0 {
		input.PacketLossPpm = 0
	}
	if input.RelayMtu < 0 {
		input.RelayMtu = 0
	}
	if input.MaxFramePayload < 0 {
		input.MaxFramePayload = 0
	}
	if input.PathDowngrades < 0 {
		input.PathDowngrades = 0
	}
	if input.PathUpgrades < 0 {
		input.PathUpgrades = 0
	}
	return input
}

func normalizeDeviceID(deviceID string) string {
	return strings.TrimSpace(deviceID)
}

func normalizeDeleteDeviceInput(input DeleteDeviceInput) DeleteDeviceInput {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}
