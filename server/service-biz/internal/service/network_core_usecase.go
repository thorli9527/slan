package service

import "context"

func (s NetworkCoreService) NetworkConfig(ctx context.Context, networkID, deviceID string) (NetworkConfigView, error) {
	networkID = normalizeNetworkID(networkID)
	deviceID = normalizeDeviceID(deviceID)
	if networkID == "" || deviceID == "" {
		return NetworkConfigView{}, ErrInvalidArgument
	}
	network, err := requireManagedNetwork(ctx, s.Networks, networkID)
	if err != nil {
		return NetworkConfigView{}, err
	}
	device, err := requireManagedDevice(ctx, s.Devices, deviceID)
	if err != nil {
		return NetworkConfigView{}, err
	}
	return buildNetworkConfigView(ctx, s.Devices, s.Networks, network, device)
}
