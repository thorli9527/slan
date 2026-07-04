package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s NetworkCoreService) ListNetworks(ctx context.Context, ownerID string) ([]NetworkSummaryView, error) {
	ownerID = normalizeNetworkOwnerID(ownerID)
	if ownerID == "" {
		return []NetworkSummaryView{}, nil
	}
	items, err := s.Networks.ListNetworksByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]NetworkSummaryView, 0, len(items))
	for _, item := range items {
		view, err := s.summarizeNetwork(ctx, item)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s NetworkCoreService) CreateNetwork(ctx context.Context, input CreateNetworkInput) (NetworkSummaryView, error) {
	input = normalizeCreateNetworkInput(input)
	if input.OwnerID == "" || input.Name == "" {
		return NetworkSummaryView{}, ErrInvalidArgument
	}
	if _, err := requireNetworkUser(ctx, s.Users, input.OwnerID); err != nil {
		return NetworkSummaryView{}, err
	}
	if input.ActorUserID != "" {
		if _, err := requireNetworkUser(ctx, s.Users, input.ActorUserID); err != nil {
			return NetworkSummaryView{}, err
		}
		if input.ActorUserID != input.OwnerID {
			return NetworkSummaryView{}, ErrUnauthorized
		}
	}
	now := networkNow(s.Now).Unix()
	item := newManagedNetwork(now, newManagedNetworkID(s.NewNetworkID), input)
	if err := s.Networks.SaveNetwork(ctx, item); err != nil {
		return NetworkSummaryView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.Broadcaster, s.Now, item.NetworkID, "network_created")
	if err != nil {
		return NetworkSummaryView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.Broadcaster, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return NetworkSummaryView{}, err
	}
	return s.summarizeNetwork(ctx, item)
}

func (s NetworkCoreService) UpdateNetwork(ctx context.Context, input UpdateNetworkInput) (NetworkSummaryView, error) {
	input = normalizeUpdateNetworkInput(input)
	if input.NetworkID == "" {
		return NetworkSummaryView{}, ErrInvalidArgument
	}
	item, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID)
	if err != nil {
		return NetworkSummaryView{}, err
	}
	item = applyUpdateNetworkInput(item, input, networkNow(s.Now).Unix())
	if err := s.Networks.SaveNetwork(ctx, item); err != nil {
		return NetworkSummaryView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.Broadcaster, s.Now, item.NetworkID, "network_updated")
	if err != nil {
		return NetworkSummaryView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.Broadcaster, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return NetworkSummaryView{}, err
	}
	return s.summarizeNetwork(ctx, item)
}

func (s NetworkCoreService) DeleteNetwork(ctx context.Context, input DeleteNetworkInput) error {
	input = normalizeDeleteNetworkInput(input)
	if input.NetworkID == "" {
		return ErrInvalidArgument
	}
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID); err != nil {
		return err
	}
	return s.Networks.DeleteNetwork(ctx, input.NetworkID)
}

func (s NetworkCoreService) summarizeNetwork(ctx context.Context, item model.Network) (NetworkSummaryView, error) {
	return buildNetworkSummaryView(ctx, s.Devices, s.Networks, item)
}

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
	return buildNetworkConfigView(ctx, s.Users, s.Devices, s.Networks, network, device)
}
