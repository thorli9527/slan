package service

import "github.com/slan/service-biz/internal/model"

func opsRelayNodeViews(items []model.RelayNode) []OpsRelayNodeView {
	views := make([]OpsRelayNodeView, 0, len(items))
	for _, item := range items {
		views = append(views, opsRelayNodeViewFromModel(item))
	}
	return views
}

func opsPunchNodeViews(items []model.PunchNode) []OpsPunchNodeView {
	views := make([]OpsPunchNodeView, 0, len(items))
	for _, item := range items {
		views = append(views, opsPunchNodeViewFromModel(item))
	}
	return views
}
