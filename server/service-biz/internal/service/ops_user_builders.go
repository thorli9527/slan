package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func buildOpsUserViews(
	ctx context.Context,
	users []model.User,
	devices repository.DeviceRepository,
) []OpsUserView {
	items := make([]OpsUserView, 0, len(users))
	for _, user := range users {
		items = append(items, buildOpsUserView(ctx, devices, user))
	}
	return items
}

func buildOpsUserView(ctx context.Context, devices repository.DeviceRepository, user model.User) OpsUserView {
	view := OpsUserView{User: opsUserRecord(user)}
	ownedDevices, err := devices.ListDevicesByOwner(ctx, user.UserID)
	if err == nil {
		view.OwnDevices = len(ownedDevices)
		var totalBytes int64
		for _, device := range ownedDevices {
			totalBytes += device.RXBytesTotal + device.TXBytesTotal
		}
		view.RelayUsedGB = int(totalBytes / (1024 * 1024 * 1024))
	}
	return view
}

func opsUserRecord(item model.User) OpsUserRecord {
	return OpsUserRecord{
		UserID:    item.UserID,
		Email:     item.Email,
		Name:      item.Name,
		Country:   item.Country,
		Province:  item.Province,
		City:      item.City,
		IPRegion:  item.IPRegion,
		Status:    item.Status,
		UpdatedAt: item.UpdatedAt,
	}
}
