package service

import "strings"

func normalizeRenewDeviceSessionInput(input RenewDeviceSessionInput) RenewDeviceSessionInput {
	input.RefreshToken = strings.TrimSpace(input.RefreshToken)
	return input
}

func normalizeDeviceAccessToken(accessToken string) string {
	return strings.TrimSpace(accessToken)
}
