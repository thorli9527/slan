package service

import "github.com/slan/service-biz/internal/model"

func opsOperatorViews(items []model.Operator) []OpsOperatorView {
	views := make([]OpsOperatorView, 0, len(items))
	for _, item := range items {
		views = append(views, opsOperatorView(item))
	}
	return views
}
