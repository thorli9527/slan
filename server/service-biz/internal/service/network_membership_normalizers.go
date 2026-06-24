package service

import "strings"

func normalizeAddNetworkDeviceInput(input AddNetworkDeviceInput) AddNetworkDeviceInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Alias = strings.TrimSpace(input.Alias)
	return input
}

func normalizeUpdateNetworkDeviceInput(input UpdateNetworkDeviceInput) UpdateNetworkDeviceInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Alias = strings.TrimSpace(input.Alias)
	input.Status = strings.TrimSpace(input.Status)
	return input
}

func normalizeNetworkDeviceScope(networkID, deviceID string) (string, string) {
	return strings.TrimSpace(networkID), strings.TrimSpace(deviceID)
}

func normalizeRemoveNetworkDeviceInput(input RemoveNetworkDeviceInput) RemoveNetworkDeviceInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}
