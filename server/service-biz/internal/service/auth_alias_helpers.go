package service

import "github.com/slan/service-biz/internal/model"

func findUserAliasCreatedAt(items []model.UserAlias, email string) int64 {
	for _, item := range items {
		if item.Email == email {
			return item.CreatedAt
		}
	}
	return 0
}
