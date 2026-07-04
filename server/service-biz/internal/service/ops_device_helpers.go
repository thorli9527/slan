package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func managedDeviceGlobalIP(ctx context.Context, networks repository.NetworkRepository, deviceID string, network model.Network) string {
	_ = ctx
	_ = networks
	_ = network
	return deviceGlobalIP(model.Device{DeviceID: deviceID})
}
