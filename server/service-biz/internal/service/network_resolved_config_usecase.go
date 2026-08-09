package service

import "context"

func (s NetworkCoreService) ResolvedNetworkConfig(ctx context.Context, networkID, deviceID string) (NetworkResolvedConfigView, error) {
	config, err := s.NetworkConfig(ctx, networkID, deviceID)
	if err != nil {
		return NetworkResolvedConfigView{}, err
	}
	relayCandidates, err := relayCandidates(ctx, s.Networks, s.RuntimeNodes, s.Now, networkID)
	if err != nil {
		return NetworkResolvedConfigView{}, err
	}
	return NetworkResolvedConfigView{
		Config:          config,
		RelayCandidates: relayCandidates,
	}, nil
}

func (s NetworkCoreService) ResolvedDeviceNetworkConfigs(ctx context.Context, deviceID string) ([]NetworkResolvedConfigView, error) {
	deviceID = normalizeDeviceID(deviceID)
	if deviceID == "" {
		return []NetworkResolvedConfigView{}, ErrInvalidArgument
	}
	networks, err := s.Networks.ListNetworksByDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	items := make([]NetworkResolvedConfigView, 0, len(networks))
	for _, network := range networks {
		if network.NetworkID == "" {
			continue
		}
		item, err := s.ResolvedNetworkConfig(ctx, network.NetworkID, deviceID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
