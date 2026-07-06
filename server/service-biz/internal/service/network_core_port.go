package service

import "context"

type NetworkCoreUseCase interface {
	ListNetworks(ctx context.Context, ownerID string) ([]NetworkSummaryView, error)
	CreateNetwork(ctx context.Context, input CreateNetworkInput) (NetworkSummaryView, error)
	UpdateNetwork(ctx context.Context, input UpdateNetworkInput) (NetworkSummaryView, error)
	DeleteNetwork(ctx context.Context, input DeleteNetworkInput) error
	NetworkConfig(ctx context.Context, networkID, deviceID string) (NetworkConfigView, error)
	NetworkSnapshot(ctx context.Context, networkID, deviceID string) (NetworkSnapshotResponse, error)
	ResolvedNetworkConfig(ctx context.Context, networkID, deviceID string) (NetworkResolvedConfigView, error)
	ResolvedDeviceNetworkConfigs(ctx context.Context, deviceID string) ([]NetworkResolvedConfigView, error)
}
