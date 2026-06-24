package service

import (
	"context"
	"github.com/slan/service-biz/internal/repository"
	"sort"
	"strings"
)

func runtimePathForDevice(ctx context.Context, networks repository.NetworkRepository, networkID, deviceID string) (NetworkRuntimePathView, bool, error) {
	networkID = normalizeNetworkID(networkID)
	deviceID = normalizeDeviceID(deviceID)
	if networkID == "" || deviceID == "" {
		return NetworkRuntimePathView{}, false, ErrInvalidArgument
	}
	items, err := networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return NetworkRuntimePathView{}, false, err
	}
	for _, item := range items {
		if item.DeviceID != deviceID {
			continue
		}
		return networkRuntimePathView(item, true), true, nil
	}
	return NetworkRuntimePathView{}, false, nil
}

func runtimePathForNode(ctx context.Context, networks repository.NetworkRepository, networkID, nodeID string) (NetworkRuntimePathView, bool, error) {
	nodeID = strings.TrimSpace(nodeID)
	deviceID := strings.TrimPrefix(nodeID, "node-")
	return runtimePathForDevice(ctx, networks, networkID, deviceID)
}

func orderRelayCandidatesByRuntime(runtime NetworkRuntimePathView, items []RelayCandidateView) []RelayCandidateView {
	if len(items) < 2 {
		return items
	}
	ordered := append([]RelayCandidateView(nil), items...)
	sortRelayCandidatesByRuntime(runtime, ordered)
	return ordered
}

func sortRelayCandidatesByRuntime(runtime NetworkRuntimePathView, items []RelayCandidateView) {
	if len(items) < 2 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := relayCandidateRuntimeRank(runtime, items[i])
		right := relayCandidateRuntimeRank(runtime, items[j])
		if left != right {
			return left < right
		}
		if items[i].RegionID != items[j].RegionID {
			return items[i].RegionID < items[j].RegionID
		}
		return items[i].EndpointID < items[j].EndpointID
	})
}

func relayCandidateRuntimeRank(runtime NetworkRuntimePathView, item RelayCandidateView) int {
	if relayCandidateMatchesRuntimeState(runtime, item) {
		return 0
	}
	switch runtime.ActivePath {
	case "relay_udp":
		if item.Transport == "udp" {
			return 10
		}
	case "derp_tcp_tls_443":
		if item.Transport == "derp_tcp_tls_443" {
			return 10
		}
	}
	if runtime.PathScore > 0 {
		return 50
	}
	return 100
}

func relayCandidateMatchesRuntimeState(runtime NetworkRuntimePathView, item RelayCandidateView) bool {
	if strings.TrimSpace(runtime.RelayEndpoint) != "" && strings.TrimSpace(runtime.RelayEndpoint) != strings.TrimSpace(item.Address) {
		return false
	}
	if strings.TrimSpace(runtime.RelayTransport) != "" && strings.TrimSpace(runtime.RelayTransport) != strings.TrimSpace(item.Transport) {
		return false
	}
	if strings.TrimSpace(runtime.DerpNodeID) != "" && strings.TrimSpace(runtime.DerpNodeID) != strings.TrimSpace(item.EndpointID) {
		return false
	}
	return strings.TrimSpace(runtime.RelayEndpoint) != "" || strings.TrimSpace(runtime.DerpNodeID) != ""
}

func applyRuntimeSelection(candidate *RelayCandidateView, runtime NetworkRuntimePathView) {
	if candidate == nil {
		return
	}
	if runtime.ObservedRttMs > 0 {
		candidate.ObservedRttMs = runtime.ObservedRttMs
	}
	if runtime.PathScore > 0 {
		candidate.PathScore = runtime.PathScore
	}
	if relayCandidateMatchesRuntimeState(runtime, *candidate) {
		candidate.Selected = true
		candidate.Reachable = true
	}
}
