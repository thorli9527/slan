package service

import "github.com/slan/service-biz/internal/model"

func bootstrapKeyViews(items []model.DeviceBootstrapKey) []DeviceBootstrapKeyView {
	views := make([]DeviceBootstrapKeyView, 0, len(items))
	for _, item := range items {
		views = append(views, bootstrapKeyView(item))
	}
	return views
}
