package impl

import (
	"context"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s *dbState) applyLiveDeviceNetworkStates(ctx context.Context, assignments []dto.NetworkAssignment) {
	for index := range assignments {
		item := &assignments[index]
		state, ok := s.loadFreshDeviceNetworkState(ctx, item.DeviceID, item.NetworkID)
		if !ok {
			item.RuntimeControlReachable = false
			item.RuntimeNetworkOnline = false
			item.RuntimeTunnelUp = false
			item.RuntimeStateFresh = false
			applyRuntimeAssignmentSummary(item)
			continue
		}
		item.RuntimeControlReachable = state.ControlReachable
		item.RuntimeNetworkOnline = state.NetworkOnline
		item.RuntimeTunnelUp = state.TunnelUp
		item.RuntimeVirtualIP = state.VirtualIP
		item.RuntimeLastSeenAt = state.LastSeenAt
		item.RuntimeStateFresh = true
		applyRuntimeAssignmentSummary(item)
	}
}

func (s *dbState) loadFreshDeviceNetworkState(ctx context.Context, deviceID, networkID string) (repo.DeviceNetworkState, bool) {
	if state, ok, err := s.tokens.LoadDeviceNetworkState(ctx, deviceID, networkID); err == nil && ok {
		if deviceNetworkStateIsFresh(state, time.Now()) {
			return state, true
		}
	}
	return repo.DeviceNetworkState{}, false
}

func (s *dbState) listFreshOnlineDeviceNetworkStates(ctx context.Context, now time.Time) []repo.DeviceNetworkState {
	states, err := s.tokens.ListDeviceNetworkStates(ctx)
	if err != nil {
		return nil
	}
	out := make([]repo.DeviceNetworkState, 0, len(states))
	for _, state := range states {
		if state.NetworkOnline && deviceNetworkStateIsFresh(state, now) {
			out = append(out, state)
		}
	}
	return out
}

func deviceNetworkStateIsFresh(state repo.DeviceNetworkState, now time.Time) bool {
	return state.LastSeenAt > 0 && now.Unix()-state.LastSeenAt <= int64(deviceNetworkStateFreshnessWindow/time.Second)
}

func applyRuntimeAssignmentSummary(item *dto.NetworkAssignment) {
	status := strings.ToLower(strings.TrimSpace(item.Status))
	item.RuntimeDeviceDisabled = status == "disabled" || status == "suspended" || status == "rejected"
	item.RuntimeHeartbeatOnline = !item.RuntimeDeviceDisabled &&
		strings.TrimSpace(item.VirtualIP) != "" &&
		item.RuntimeStateFresh &&
		item.RuntimeControlReachable
	item.RuntimeIPApplied = item.RuntimeHeartbeatOnline &&
		strings.TrimSpace(item.RuntimeVirtualIP) != "" &&
		strings.TrimSpace(item.RuntimeVirtualIP) == strings.TrimSpace(item.VirtualIP)
	item.RuntimeNetworkEnabled = item.RuntimeHeartbeatOnline &&
		item.RuntimeNetworkOnline &&
		item.RuntimeTunnelUp &&
		item.RuntimeIPApplied
}
