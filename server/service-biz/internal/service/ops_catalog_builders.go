package service

import "github.com/slan/service-biz/internal/model"

func planViews(items []model.Plan) []PlanView {
	views := make([]PlanView, 0, len(items))
	for _, item := range items {
		views = append(views, planView(item))
	}
	return views
}

func productViews(items []model.Product) []ProductView {
	views := make([]ProductView, 0, len(items))
	for _, item := range items {
		views = append(views, productView(item))
	}
	return views
}
