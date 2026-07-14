package service

import "strings"

func normalizeNetworkDeviceScope(networkID, deviceID string) (string, string) {
	return strings.TrimSpace(networkID), strings.TrimSpace(deviceID)
}
