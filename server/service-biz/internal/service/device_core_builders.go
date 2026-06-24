package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func deviceViews(items []model.Device) []DeviceView {
	views := make([]DeviceView, 0, len(items))
	for _, item := range items {
		views = append(views, deviceView(item))
	}
	return views
}

func networkSummaryViews(items []model.Network) []NetworkSummaryView {
	views := make([]NetworkSummaryView, 0, len(items))
	for _, item := range items {
		views = append(views, networkSummaryView(item))
	}
	return views
}

func buildDeviceProfiles(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	items []model.Device,
) ([]DeviceProfileView, error) {
	views := make([]DeviceProfileView, 0, len(items))
	for _, item := range items {
		view, err := buildDeviceProfile(ctx, users, networks, item)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}
