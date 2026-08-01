package service

import "github.com/slan/service-biz/internal/model"

func userSummaryViews(items []model.User) []UserSummaryView {
	views := make([]UserSummaryView, 0, len(items))
	for _, item := range items {
		views = append(views, userSummaryView(item))
	}
	return views
}
