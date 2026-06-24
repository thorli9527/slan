package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func buildDeviceInviteViews(ctx context.Context, users repository.UserRepository, devices repository.DeviceRepository, items []model.DeviceInvite) ([]DeviceInviteView, error) {
	views := make([]DeviceInviteView, 0, len(items))
	for _, item := range items {
		view, err := buildDeviceInviteView(ctx, users, devices, item)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func buildNetworkDeviceViews(ctx context.Context, devices repository.DeviceRepository, items []model.NetworkDevice) ([]NetworkDeviceView, error) {
	views := make([]NetworkDeviceView, 0, len(items))
	for _, item := range items {
		view, err := buildNetworkDeviceView(ctx, devices, item)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}
