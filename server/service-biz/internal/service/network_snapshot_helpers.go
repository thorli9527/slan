package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/repository"
)

func buildNetworkSnapshotPayload(
	ctx context.Context,
	users repository.UserRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	ops repository.OpsRepository,
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
		networkDevices = append(networkDevices, member.DeviceID)
	}
	primaryDeviceID := ""
	if len(networkDevices) > 0 {
		primaryDeviceID = strings.TrimSpace(networkDevices[0])
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

func publishNetworkSnapshot(
	ctx context.Context,
	users repository.UserRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	ops repository.OpsRepository,
	broadcaster networkBroadcastPublisher,
	nowFn func() time.Time,
	networkID string,
	version int64,
	reason string,
) error {
	if broadcaster == nil {
		return nil
	}
	snapshot, err := buildNetworkSnapshotPayload(ctx, users, devices, networks, ops, nowFn, networkID)
	if err != nil {
		return err
	}
	return broadcaster.PublishNetworkSnapshot(ctx, networkBroadcastSnapshot{
		NetworkID:  strings.TrimSpace(networkID),
		Version:    version,
		Reason:     strings.TrimSpace(reason),
		Snapshot:   snapshot,
		OccurredAt: currentTime(nowFn),
	})
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
		"deviceGroupsByDevice": view.DeviceGroupsByDevice,
		"runtimePath":          view.RuntimePath,
		"peers":                peers,
		"dnsZones":             view.DNSZones,
		"dnsRecords":           view.DNSRecords,
		"publicMappings":       view.PublicMappings,
		"securityGroups":       view.SecurityGroups,
		"securityRules":        view.SecurityRules,
		"relayCandidates":      relayCandidates,
	}
}
