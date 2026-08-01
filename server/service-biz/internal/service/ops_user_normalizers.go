package service

import "strings"

func normalizeCreateUserInput(input CreateUserInput) CreateUserInput {
	input.Email = normalizedEmail(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	return input
}

func normalizeSetUserPasswordInput(input SetUserPasswordInput) SetUserPasswordInput {
	input.UserID = strings.TrimSpace(input.UserID)
	return input
}

func normalizeUpdateUserInput(input UpdateUserInput) UpdateUserInput {
	input.UserID = strings.TrimSpace(input.UserID)
	input.Email = strings.TrimSpace(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	input.Country = strings.TrimSpace(input.Country)
	input.Province = strings.TrimSpace(input.Province)
	input.City = strings.TrimSpace(input.City)
	input.IPRegion = strings.TrimSpace(input.IPRegion)
	input.Status = strings.TrimSpace(input.Status)
	return input
}
