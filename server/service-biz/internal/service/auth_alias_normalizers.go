package service

import "strings"

func normalizeUserAliasInput(input UpsertUserAliasInput) UpsertUserAliasInput {
	input.UserID = strings.TrimSpace(input.UserID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Email = normalizedEmail(input.Email)
	input.Alias = strings.TrimSpace(input.Alias)
	return input
}
