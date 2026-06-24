package service

import "github.com/slan/service-biz/internal/model"

func userAliasView(item model.UserAlias) UserAliasView {
	return UserAliasView{
		UserID:    item.UserID,
		Email:     item.Email,
		Alias:     item.Alias,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}
