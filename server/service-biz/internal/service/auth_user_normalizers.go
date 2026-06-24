package service

import "strings"

func normalizeRegisterUserInput(input RegisterUserInput) RegisterUserInput {
	input.Email = normalizedEmail(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	return input
}

func normalizeLoginUserInput(input LoginUserInput) LoginUserInput {
	input.Email = normalizedEmail(input.Email)
	return input
}

func normalizeUserAccessToken(accessToken string) string {
	return strings.TrimSpace(accessToken)
}

func normalizeUserID(userID string) string {
	return strings.TrimSpace(userID)
}

func normalizeChangeUserPasswordInput(input ChangeUserPasswordInput) ChangeUserPasswordInput {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}
