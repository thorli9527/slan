package service

import "strings"

func normalizeCreateConsoleLoginKeyInput(input CreateConsoleLoginKeyInput) CreateConsoleLoginKeyInput {
	input.UserID = strings.TrimSpace(input.UserID)
	return input
}

func normalizeConsoleLoginInput(input ConsoleLoginInput) ConsoleLoginInput {
	input.Key = strings.TrimSpace(input.Key)
	input.LoginKey = strings.TrimSpace(input.LoginKey)
	return input
}
