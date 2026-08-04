package service

import "context"

type NetworkCoreUseCase interface {
	NetworkConfig(ctx context.Context, networkID, deviceID string) (NetworkConfigView, error)
	NetworkSnapshot(ctx context.Context, networkID, deviceID string) (NetworkSnapshotResponse, error)
	ResolvedNetworkConfig(ctx context.Context, networkID, deviceID string) (NetworkResolvedConfigView, error)
	ResolvedDeviceNetworkConfigs(ctx context.Context, deviceID string) ([]NetworkResolvedConfigView, error)
}
