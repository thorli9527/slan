package service

import "github.com/slan/service-biz/internal/model"

func userAliasViews(items []model.UserAlias) []UserAliasView {
	views := make([]UserAliasView, 0, len(items))
	for _, item := range items {
		views = append(views, userAliasView(item))
	}
	return views
}
