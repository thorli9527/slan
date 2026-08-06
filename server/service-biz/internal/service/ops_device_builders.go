package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func buildManagedDeviceView(
	ctx context.Context,
	networks repository.NetworkRepository,
	runtime repository.DeviceRuntimeRepository,
	device model.Device,
) (OpsManagedDeviceView, error) {
	view := OpsManagedDeviceView{
		Device:        deviceView(device),
		GlobalIP:      deviceGlobalIP(device),
		GlobalName:    networkGlobalName(device.DeviceID, device.Alias, device.Name),
		DeviceEnabled: device.Status == "active",
	}
	if runtime != nil {
		state, ok, err := runtime.GetDeviceRuntime(ctx, device.DeviceID)
		if err != nil {
			return OpsManagedDeviceView{}, err
		}
		if ok {
			view.HeartbeatOnline = deviceRuntimeOnline(state)
			view.NetworkEnabled = state.NetworkEnabled
		}
	}
	attachedNetworks, err := networks.ListNetworksByDevice(ctx, device.DeviceID)
	if err != nil || len(attachedNetworks) == 0 {
		return view, err
	}
	view.NetworkCount = len(attachedNetworks)
	return view, nil
}

func deviceRuntimeOnline(state model.DeviceRuntimeState) bool {
	return state.ApplicationState == "running"
}
