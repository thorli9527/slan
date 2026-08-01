package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func buildNetworkSnapshotPayload(
	ctx context.Context,
	users repository.UserRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	ops repository.OpsNodeRepository,
	nowFn func() time.Time,
	networkID string,
) (map[string]any, error) {
	networkID = strings.TrimSpace(networkID)
	if networkID == "" {
		return map[string]any{}, ErrInvalidArgument
	}
	memberships, err := networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return nil, err
	}
	networkDevices := make([]string, 0, len(memberships))
	for _, member := range memberships {
		if !networkMemberActive(member) {
			continue
		}
		deviceID := strings.TrimSpace(member.DeviceID)
		if deviceID == "" {
			continue
		}
		networkDevices = append(networkDevices, deviceID)
	}
	primaryDeviceID, err := firstActiveManagedNetworkDeviceID(ctx, devices, memberships)
	if err != nil {
		return nil, err
	}
	if primaryDeviceID == "" {
		return map[string]any{
			"networkId": networkID,
			"devices":   []string{},
		}, nil
	}
	core := NetworkCoreService{
		Users:    users,
		Devices:  devices,
		Networks: networks,
		Ops:      ops,
		Now:      nowFn,
	}
	resolved, err := core.ResolvedNetworkConfig(ctx, networkID, primaryDeviceID)
	if err != nil {
		return nil, err
	}
	return networkResolvedSnapshotPayload(resolved), nil
}

func firstActiveManagedNetworkDeviceID(
	ctx context.Context,
	devices repository.DeviceRepository,
	memberships []model.NetworkDevice,
) (string, error) {
	if devices == nil {
		return "", nil
	}
	for _, member := range memberships {
		if !networkMemberActive(member) {
			continue
		}
		deviceID := strings.TrimSpace(member.DeviceID)
		if deviceID == "" {
			continue
		}
		_, ok, err := devices.GetDevice(ctx, deviceID)
		if err != nil {
			return "", err
		}
		if ok {
			return deviceID, nil
		}
	}
	return "", nil
}

func publishNetworkSnapshot(
	ctx context.Context,
	users repository.UserRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	ops repository.OpsNodeRepository,
	eventPublisher NetworkEventPublisher,
	nowFn func() time.Time,
	networkID string,
	version int64,
	reason string,
) error {
	if eventPublisher == nil {
		return nil
	}
	snapshot, err := buildNetworkSnapshotPayload(ctx, users, devices, networks, ops, nowFn, networkID)
	if err != nil {
		return err
	}
	occurredAt := currentTime(nowFn)
	eventSnapshot, err := buildNetworkEventSnapshotFromRepositories(
		ctx,
		users,
		devices,
		networks,
		ops,
		nowFn,
		networkID,
	)
	if err != nil {
		return err
	}
	_ = snapshot
	return publishNetworkEvent(
		ctx,
		eventPublisher,
		NetworkEventSnapshot,
		networkID,
		uint64(version),
		occurredAt.UnixMilli(),
		eventSnapshot,
	)
}

func networkResolvedSnapshotPayload(resolved NetworkResolvedConfigView) map[string]any {
	view := resolved.Config
	relayCandidates := make([]map[string]any, 0, len(resolved.RelayCandidates))
	for _, candidate := range resolved.RelayCandidates {
		relayCandidates = append(relayCandidates, map[string]any{
			"endpointId":    candidate.EndpointID,
			"transport":     candidate.Transport,
			"address":       candidate.Address,
			"countryCode":   candidate.CountryCode,
			"regionId":      candidate.RegionID,
			"clusterId":     candidate.ClusterID,
			"reachable":     candidate.Reachable,
			"observedRttMs": candidate.ObservedRttMs,
			"pathScore":     candidate.PathScore,
			"selected":      candidate.Selected,
		})
	}
	peers := make([]map[string]any, 0, len(view.Peers))
	for _, peer := range view.Peers {
		peers = append(peers, map[string]any{
			"deviceId":   peer.DeviceID,
			"ownerId":    peer.OwnerID,
			"ownerEmail": peer.OwnerEmail,
			"alias":      peer.Alias,
			"globalIp":   peer.GlobalIP,
			"globalName": peer.GlobalName,
			"status":     peer.Status,
			"endpoints":  peer.Endpoints,
		})
	}
	return map[string]any{
		"network":              view.Network,
		"deviceId":             view.DeviceID,
		"nodeId":               view.NodeID,
		"globalIp":             view.GlobalIP,
		"prefixLen":            view.PrefixLen,
		"globalName":           view.GlobalName,
		"resolver":             BuildNetworkDNSConfigView(view),
		"deviceGroupsByDevice": view.DeviceGroupsByDevice,
		"runtimePath":          view.RuntimePath,
		"peers":                peers,
		"resolverZones":        view.DNSZones,
		"resolverRecords":      view.DNSRecords,
		"securityGroups":       view.SecurityGroups,
		"securityRules":        view.SecurityRules,
		"relayCandidates":      relayCandidates,
	}
}
