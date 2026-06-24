package service

import "github.com/slan/service-biz/internal/model"

func orderViews(items []model.Order) []OrderView {
	views := make([]OrderView, 0, len(items))
	for _, item := range items {
		views = append(views, orderView(item))
	}
	return views
}

func renewalViews(items []model.Renewal) []RenewalView {
	views := make([]RenewalView, 0, len(items))
	for _, item := range items {
		views = append(views, renewalView(item))
	}
	return views
}
