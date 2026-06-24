package service

import "strings"

func normalizeBindDeviceSessionInput(input BindDeviceSessionInput) BindDeviceSessionInput {
	input.UserID = strings.TrimSpace(input.UserID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
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

func normalizeRenewDeviceSessionInput(input RenewDeviceSessionInput) RenewDeviceSessionInput {
	return input
}

func normalizeDeviceAccessToken(accessToken string) string {
	return strings.TrimSpace(accessToken)
}
