package service

import "github.com/slan/service-biz/internal/model"

func newUserAlias(input UpsertUserAliasInput, createdAt int64, now int64) model.UserAlias {
	if createdAt == 0 {
		createdAt = now
	}
	return model.UserAlias{
		UserID:    input.UserID,
		Email:     input.Email,
		Alias:     input.Alias,
		CreatedAt: createdAt,
		UpdatedAt: now,
	}
}
