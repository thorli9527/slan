package service

import "strings"

func normalizeRegisterUserInput(input RegisterUserInput) RegisterUserInput {
	input.Email = normalizedEmail(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	input.ClientType = normalizedUserSessionClient(strings.TrimSpace(input.ClientType))
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	return input
}

func normalizeLoginUserInput(input LoginUserInput) LoginUserInput {
	input.Email = normalizedEmail(input.Email)
	input.SessionMode = normalizedSessionMode(strings.TrimSpace(input.SessionMode))
	input.ClientType = normalizedUserSessionClient(strings.TrimSpace(input.ClientType))
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	return input
}

func normalizeUserAccessToken(accessToken string) string {
	return strings.TrimSpace(accessToken)
}

func normalizeUserRefreshToken(refreshToken string) string {
	return strings.TrimSpace(refreshToken)
}

func normalizeRenewUserSessionInput(input RenewUserSessionInput) RenewUserSessionInput {
	input.RefreshToken = normalizeUserRefreshToken(input.RefreshToken)
	return input
}

func normalizeUserID(userID string) string {
	return strings.TrimSpace(userID)
}

func normalizeChangeUserPasswordInput(input ChangeUserPasswordInput) ChangeUserPasswordInput {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}
