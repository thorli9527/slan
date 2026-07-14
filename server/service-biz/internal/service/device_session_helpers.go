package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func replaceDeviceSession(
	ctx context.Context,
	devices repository.DeviceRepository,
	session model.DeviceSession,
) error {
	items, err := devices.ListDeviceSessionsByDeviceID(ctx, session.DeviceID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.SessionID == session.SessionID {
			continue
		}
		if err := devices.DeleteDeviceSessionByAccessToken(ctx, item.AccessToken); err != nil {
			return err
		}
	}
	return devices.SaveDeviceSession(ctx, session)
}
