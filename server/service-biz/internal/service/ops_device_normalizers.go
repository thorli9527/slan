package service

import "strings"

func normalizeUpdateDeviceInput(input UpdateDeviceInput) UpdateDeviceInput {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.Name = strings.TrimSpace(input.Name)
	input.Alias = strings.TrimSpace(input.Alias)
	input.Status = strings.TrimSpace(input.Status)
	return input
}

func normalizeManagedDeviceID(deviceID string) string {
	return strings.TrimSpace(deviceID)
}
