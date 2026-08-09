package service

import "github.com/slan/service-biz/internal/model"

func runtimeRelayNodeViews(items []model.RelayNode) []RuntimeRelayNodeView {
	views := make([]RuntimeRelayNodeView, 0, len(items))
	for _, item := range items {
		views = append(views, runtimeRelayNodeViewFromModel(item))
	}
	return views
}

func runtimePunchNodeViews(items []model.PunchNode) []RuntimePunchNodeView {
	views := make([]RuntimePunchNodeView, 0, len(items))
	for _, item := range items {
		views = append(views, runtimePunchNodeViewFromModel(item))
	}
	return views
}
