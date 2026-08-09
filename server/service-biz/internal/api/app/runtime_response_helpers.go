package app

import (
	"context"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func runtimeSessionPayload(
	ctx context.Context,
	runtime servicepkg.NetworkRuntimeUseCase,
	networks servicepkg.NetworkCoreUseCase,
	serverNodes servicepkg.OpsServerNodeUseCase,
	deviceID string,
	build func([]servicepkg.PunchNodeView, []servicepkg.OpsServerNodeView, []map[string]any) map[string]any,
) map[string]any {
	punchNodes, _ := runtime.ListPunchNodes(ctx)
	proxyNodes, _ := serverNodes.ListServerNodes(ctx)
	networkConfigs := buildDeviceNetworkConfigPayloads(ctx, networks, deviceID)
	return build(punchNodes, proxyNodes, networkConfigs)
}

func renewedDeviceRuntimePayload(
	ctx context.Context,
	networks servicepkg.NetworkCoreUseCase,
	view servicepkg.DeviceProfileView,
) map[string]any {
	networkConfigs := buildDeviceNetworkConfigPayloads(ctx, networks, view.Device.DeviceID)
	return renewedDevicePayloadWithConfigs(view, networkConfigs)
}
