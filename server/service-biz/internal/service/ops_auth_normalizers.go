package service

import "strings"

func normalizeCreateOperatorInput(input CreateOperatorInput) CreateOperatorInput {
	input.Email = normalizedEmail(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	input.Role = strings.TrimSpace(input.Role)
	return input
}

func normalizeUpdateOperatorInput(input UpdateOperatorInput) UpdateOperatorInput {
	input.OperatorID = strings.TrimSpace(input.OperatorID)
	input.Name = strings.TrimSpace(input.Name)
	input.Role = strings.TrimSpace(input.Role)
	input.Status = strings.TrimSpace(input.Status)
	return input
}

func normalizeSetOperatorPasswordInput(input SetOperatorPasswordInput) SetOperatorPasswordInput {
	input.OperatorID = strings.TrimSpace(input.OperatorID)
	return input
}
