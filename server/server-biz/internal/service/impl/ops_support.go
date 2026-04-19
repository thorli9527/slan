package impl

import (
	"context"
	"math"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
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
	clusters := s.state.relayClusters()
	clusterCount := 0
	relayNodeCount := 0
	for _, cluster := range clusters {
		clusterCount++
		relayNodeCount += len(cluster.nodes)
	}
	onlineCount := 0
	for _, device := range devices {
		if device.Status == "online" {
			onlineCount++
		}
	}
	return dto.OpsOverview{
		UserCount:         len(users),
		DeviceCount:       len(devices),
		OnlineDeviceCount: onlineCount,
		NodeCount:         len(nodes),
		RelayClusterCount: clusterCount,
		RelayNodeCount:    relayNodeCount,
	}, nil
}

func (s dbOpsService) ListUsers() ([]dto.OpsUser, error) {
	ctx := context.Background()
	users, err := s.state.pg.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	devices, err := s.state.pg.ListDevices(ctx)
	if err != nil {
		return nil, err
	}
	nodes, err := s.state.pg.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := s.state.pg.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	userRoles, err := s.state.pg.ListUserRoles(ctx)
	if err != nil {
		return nil, err
	}
	deviceCountByUser := make(map[string]int, len(users))
	nodeCountByUser := make(map[string]int, len(users))
	roleByID := make(map[string]repo.Role, len(roles))
	for _, role := range roles {
		roleByID[role.RoleID] = role
	}
	roleIDsByUser := make(map[string][]string)
	roleCodesByUser := make(map[string][]string)
	roleNamesByUser := make(map[string][]string)
	for _, device := range devices {
		deviceCountByUser[device.UserID]++
	}
	for _, node := range nodes {
		nodeCountByUser[node.UserID]++
	}
	for _, binding := range userRoles {
		roleIDsByUser[binding.UserID] = append(roleIDsByUser[binding.UserID], binding.RoleID)
		if role, ok := roleByID[binding.RoleID]; ok {
			roleCodesByUser[binding.UserID] = append(roleCodesByUser[binding.UserID], role.RoleCode)
			roleNamesByUser[binding.UserID] = append(roleNamesByUser[binding.UserID], role.RoleName)
		}
	}
	out := make([]dto.OpsUser, 0, len(users))
	for _, user := range users {
		out = append(out, dto.OpsUser{
			UserID:      user.UserID,
			Email:       user.Email,
			DeviceCount: deviceCountByUser[user.UserID],
			NodeCount:   nodeCountByUser[user.UserID],
			RoleIDs:     append([]string(nil), roleIDsByUser[user.UserID]...),
			RoleCodes:   append([]string(nil), roleCodesByUser[user.UserID]...),
			RoleNames:   append([]string(nil), roleNamesByUser[user.UserID]...),
		})
	}
	return out, nil
}

func (s dbOpsService) ListDevices() ([]dto.OpsDevice, error) {
	ctx := context.Background()
	devices, err := s.state.pg.ListDevices(ctx)
	if err != nil {
		return nil, err
	}
	nodes, err := s.state.pg.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	nodeIDsByDevice := make(map[string][]string)
	for _, node := range nodes {
		nodeIDsByDevice[node.DeviceID] = append(nodeIDsByDevice[node.DeviceID], node.NodeID)
	}
	out := make([]dto.OpsDevice, 0, len(devices))
	for _, device := range devices {
		networkIDs, err := s.state.deviceNetworkIDs(ctx, device.DeviceID)
		if err != nil {
			return nil, err
		}
		item := dto.OpsDevice{
			Device: dto.Device{
				DeviceID:   device.DeviceID,
				Name:       device.Name,
				Platform:   device.Platform,
				Status:     device.Status,
				NetworkIDs: networkIDs,
			},
			UserID:    device.UserID,
			NodeCount: len(nodeIDsByDevice[device.DeviceID]),
			NodeIDs:   append([]string(nil), nodeIDsByDevice[device.DeviceID]...),
		}
		if device.PublicKey != nil {
			item.PublicKey = *device.PublicKey
		}
		out = append(out, item)
	}
	return out, nil
}

func (s dbOpsService) RelayTopology() (dto.OpsRelayTopology, error) {
	health := s.state.relayNodeHealth(context.Background(), time.Now())
	nodes := make([]dto.OpsRelayNode, 0)
	for _, cluster := range s.state.relayClusters() {
		for _, node := range cluster.nodes {
			item := dto.OpsRelayNode{
				NodeID:      node.NodeID,
				ClusterID:   cluster.clusterID,
				ClusterName: cluster.clusterName,
				CountryCode: cluster.countryCode,
				CountryName: cluster.countryName,
				CityCode:    cluster.cityCode,
				CityName:    cluster.cityName,
				Transport:   node.Transport,
				Address:     node.Address,
				Priority:    node.Priority,
			}
			if rank, ok := health[node.NodeID]; ok {
				if rank.hasRtt {
					item.ObservedRttMs = uint32(math.Round(rank.rttAvg))
				}
				if rank.hasScore {
					item.PathScore = uint32(math.Round(rank.scoreAvg))
				}
				item.SampleCount = rank.samples
			}
			nodes = append(nodes, item)
		}
	}
	return dto.OpsRelayTopology{
		DefaultClusterID: s.state.cfg.Relay.DefaultClusterID,
		Regions:          s.state.relayRegions(),
		Nodes:            nodes,
	}, nil
}

func (s dbOpsService) ListAdmins() ([]dto.OpsAdminInfo, error) {
	ctx := context.Background()
	admins, err := s.state.pg.ListAdminInfo(ctx)
	if err != nil {
		return nil, err
	}
	users, err := s.state.pg.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := s.state.pg.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	userRoles, err := s.state.pg.ListUserRoles(ctx)
	if err != nil {
		return nil, err
	}
	userByID := make(map[string]string, len(users))
	for _, user := range users {
		userByID[user.UserID] = user.Email
	}
	roleByID := make(map[string]repo.Role, len(roles))
	for _, role := range roles {
		roleByID[role.RoleID] = role
	}
	roleIDsByUser := make(map[string][]string)
	roleCodesByUser := make(map[string][]string)
	roleNamesByUser := make(map[string][]string)
	for _, binding := range userRoles {
		roleIDsByUser[binding.UserID] = append(roleIDsByUser[binding.UserID], binding.RoleID)
		if role, ok := roleByID[binding.RoleID]; ok {
			roleCodesByUser[binding.UserID] = append(roleCodesByUser[binding.UserID], role.RoleCode)
			roleNamesByUser[binding.UserID] = append(roleNamesByUser[binding.UserID], role.RoleName)
		}
	}
	out := make([]dto.OpsAdminInfo, 0, len(admins))
	for _, admin := range admins {
		out = append(out, dto.OpsAdminInfo{
			AdminID:     admin.AdminID,
			UserID:      admin.UserID,
			Email:       userByID[admin.UserID],
			DisplayName: admin.DisplayName,
			Phone:       admin.Phone,
			Status:      admin.Status,
			RoleIDs:     append([]string(nil), roleIDsByUser[admin.UserID]...),
			RoleCodes:   append([]string(nil), roleCodesByUser[admin.UserID]...),
			RoleNames:   append([]string(nil), roleNamesByUser[admin.UserID]...),
		})
	}
	return out, nil
}
