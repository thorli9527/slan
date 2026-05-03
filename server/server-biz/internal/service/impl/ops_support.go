package impl

import (
	"context"
	"time"

	"github.com/slan/server/server-biz/api/dto"
)

func (s dbOpsService) Overview() (dto.OpsOverview, error) {
	ctx := context.Background()
	users, err := s.state.pg.ListUsers(ctx)
	if err != nil {
		return dto.OpsOverview{}, err
	}
	devices, err := s.state.pg.ListDevices(ctx)
	if err != nil {
		return dto.OpsOverview{}, err
	}
	nodes, err := s.state.pg.ListNodes(ctx)
	if err != nil {
		return dto.OpsOverview{}, err
	}
	relayOverview := s.relayOverview(ctx)
	onlineCount := s.networkOnlineDeviceCount(ctx)
	defaultAdminSeeded, defaultAdminRoleBound, err := s.defaultAdminSeedStatus(ctx)
	if err != nil {
		return dto.OpsOverview{}, err
	}
	warnings, err := s.opsSecurityWarnings(ctx)
	if err != nil {
		return dto.OpsOverview{}, err
	}
	return dto.OpsOverview{
		UserCount:             len(users),
		DeviceCount:           len(devices),
		OnlineDeviceCount:     onlineCount,
		NodeCount:             len(nodes),
		RelayClusterCount:     relayOverview.clusterCount,
		RelayNodeCount:        relayOverview.nodeCount,
		RelayOnlineNodeCount:  relayOverview.onlineNodeCount,
		DefaultAdminSeeded:    defaultAdminSeeded,
		DefaultAdminLoginName: s.state.cfg.Ops.DefaultAdmin.LoginName,
		DefaultAdminRoleBound: defaultAdminRoleBound,
		SecurityWarnings:      warnings,
	}, nil
}

func (s dbOpsService) networkOnlineDeviceCount(ctx context.Context) int {
	states := s.state.listFreshOnlineDeviceNetworkStates(ctx, time.Now())
	seen := make(map[string]struct{}, len(states))
	for _, state := range states {
		seen[state.DeviceID] = struct{}{}
	}
	return len(seen)
}
