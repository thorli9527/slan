package impl

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
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
	relayOnlineNodeCount := 0
	heartbeats := s.state.relayNodeHeartbeats(ctx, time.Now())
	for _, cluster := range clusters {
		clusterCount++
		relayNodeCount += len(cluster.nodes)
		for _, node := range cluster.nodes {
			if heartbeat, ok := heartbeats[node.NodeID]; ok && heartbeat.Healthy {
				relayOnlineNodeCount++
			}
		}
	}
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
		RelayClusterCount:     clusterCount,
		RelayNodeCount:        relayNodeCount,
		RelayOnlineNodeCount:  relayOnlineNodeCount,
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
	userPlanOverrides := make(map[string]dto.PlanConfig)
	for _, user := range users {
		if override, err := s.state.pg.GetUserPlanOverride(ctx, user.UserID); err == nil {
			userPlanOverrides[user.UserID] = dto.PlanConfig{
				MaxActiveDevices: override.MaxActiveDevices,
				RelayIngressKbps: override.RelayIngressKbps,
				RelayEgressKbps:  override.RelayEgressKbps,
				UDPIngressKbps:   override.UDPIngressKbps,
				UDPEgressKbps:    override.UDPEgressKbps,
			}
		}
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
		item := dto.OpsUser{
			UserID:      user.UserID,
			Email:       user.Email,
			DeviceCount: deviceCountByUser[user.UserID],
			NodeCount:   nodeCountByUser[user.UserID],
			RoleIDs:     append([]string(nil), roleIDsByUser[user.UserID]...),
			RoleCodes:   append([]string(nil), roleCodesByUser[user.UserID]...),
			RoleNames:   append([]string(nil), roleNamesByUser[user.UserID]...),
		}
		if override, ok := userPlanOverrides[user.UserID]; ok {
			copy := override
			item.PlanOverride = &copy
		}
		out = append(out, item)
	}
	return out, nil
}

func (s dbOpsService) PlanConfig() (dto.PlanConfig, error) {
	return s.state.globalPlanConfig(context.Background()), nil
}

func (s dbOpsService) UpdatePlanConfig(req dto.UpdatePlanConfigRequest) (dto.PlanConfig, error) {
	ctx := context.Background()
	record := repoPlanConfigFromDTO(globalPlanConfigID, dto.PlanConfig(req))
	if err := s.state.pg.UpsertPlanConfig(ctx, record); err != nil {
		return dto.PlanConfig{}, err
	}
	return s.state.globalPlanConfig(ctx), nil
}

func (s dbOpsService) UpdateUserPlanOverride(userID string, req dto.UpdatePlanConfigRequest) (dto.UserPlanOverride, error) {
	ctx := context.Background()
	userID = strings.TrimSpace(userID)
	if _, err := s.state.pg.GetUserByID(ctx, userID); err != nil {
		if repo.IsNotFound(err) {
			return dto.UserPlanOverride{}, ErrNotFound
		}
		return dto.UserPlanOverride{}, err
	}
	record := repoUserPlanOverrideFromDTO(userID, dto.PlanConfig(req))
	if err := s.state.pg.UpsertUserPlanOverride(ctx, record); err != nil {
		return dto.UserPlanOverride{}, err
	}
	return dto.UserPlanOverride{
		UserID: userID,
		PlanConfig: dto.PlanConfig{
			MaxActiveDevices: record.MaxActiveDevices,
			RelayIngressKbps: record.RelayIngressKbps,
			RelayEgressKbps:  record.RelayEgressKbps,
			UDPIngressKbps:   record.UDPIngressKbps,
			UDPEgressKbps:    record.UDPEgressKbps,
		},
	}, nil
}

func (s dbOpsService) DeleteUserPlanOverride(userID string) error {
	return s.state.pg.DeleteUserPlanOverride(context.Background(), strings.TrimSpace(userID))
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
	ctx := context.Background()
	now := time.Now()
	health := s.state.relayNodeHealth(ctx, now)
	heartbeats := s.state.relayNodeHeartbeats(ctx, now)
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
			if heartbeat, ok := heartbeats[node.NodeID]; ok {
				item.HeartbeatOnline = heartbeat.Healthy
				item.HeartbeatLastSeenAt = heartbeat.UpdatedAt
				item.ActiveSessions = heartbeat.ActiveSessions
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

func (s dbOpsService) NetworkQuality() (dto.OpsNetworkQuality, error) {
	ctx := context.Background()
	cutoff := time.Now().Add(-nodePathHealthFreshnessWindow).Unix()
	records, err := s.state.pg.ListRecentNodePathHealth(ctx, cutoff)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	nodes, err := s.state.pg.ListNodes(ctx)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	devices, err := s.state.pg.ListDevices(ctx)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	users, err := s.state.pg.ListUsers(ctx)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	networks, err := s.state.pg.ListNetworks(ctx)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	policyExecutions, err := s.state.pg.ListRecentRelayPolicyExecutions(ctx, cutoff)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	nodeByID := make(map[string]repo.Node, len(nodes))
	for _, node := range nodes {
		nodeByID[node.NodeID] = node
	}
	deviceByID := make(map[string]repo.Device, len(devices))
	for _, device := range devices {
		deviceByID[device.DeviceID] = device
	}
	emailByUserID := make(map[string]string, len(users))
	for _, user := range users {
		emailByUserID[user.UserID] = user.Email
	}
	networkNameByID := make(map[string]string, len(networks))
	for _, network := range networks {
		networkNameByID[network.NetworkID] = network.Name
	}
	policyExecutionByNetworkDevice := make(map[string]repo.RelayPolicyExecution, len(policyExecutions))
	for _, execution := range policyExecutions {
		key := execution.NetworkID + "|" + execution.DeviceID
		if current, ok := policyExecutionByNetworkDevice[key]; !ok || execution.UpdatedAt > current.UpdatedAt {
			policyExecutionByNetworkDevice[key] = execution
		}
	}

	items := make([]dto.OpsNetworkQualityItem, 0, len(records))
	for _, record := range records {
		item := dto.OpsNetworkQualityItem{
			HealthID:    record.HealthID,
			NetworkID:   record.NetworkID,
			NetworkName: networkNameByID[record.NetworkID],
			NodeID:      record.NodeID,
			PeerNodeID:  record.PeerNodeID,
			PathType:    record.PathType,
			ActivePath:  record.ActivePath,
			Endpoint:    record.Endpoint,
			DerpNodeID:  record.DerpNodeID,
			SampledAtMs: record.SampledAtMs,
			UpdatedAt:   record.UpdatedAt,
		}
		if value := record.ObservedRttMs; value != nil {
			item.ObservedRttMs = *value
		}
		if value := record.PacketLossPpm; value != nil {
			item.PacketLossPpm = *value
		}
		if value := record.PathScore; value != nil {
			item.PathScore = *value
		}
		item.SourceCountryCode = record.SourceCountryCode
		item.RelayCountryCode = record.RelayCountryCode
		item.PeerCountryCode = record.PeerCountryCode
		item.CrossCountry = record.CrossCountry
		item.PathDowngrades = record.PathDowngrades
		item.PathUpgrades = record.PathUpgrades
		item.LastPathChange = record.LastPathChange
		if value := record.RelayMtu; value != nil {
			item.RelayMtu = *value
		}
		if value := record.MaxFramePayload; value != nil {
			item.MaxFramePayload = *value
		}
		if node, ok := nodeByID[record.NodeID]; ok {
			item.UserID = node.UserID
			item.UserEmail = emailByUserID[node.UserID]
			item.DeviceID = node.DeviceID
			if device, ok := deviceByID[node.DeviceID]; ok {
				item.DeviceName = device.Name
			}
			if execution, ok := policyExecutionByNetworkDevice[record.NetworkID+"|"+node.DeviceID]; ok {
				item.PolicyID = execution.PolicyID
				item.PolicyScope = execution.Scope
				item.PolicyApplied = execution.Applied
				item.PolicyReportedAtMs = execution.ReportedAtMs
			}
		}
		items = append(items, item)
	}
	return dto.OpsNetworkQuality{Items: items, Summary: buildOpsNetworkQualitySummary(records, networkNameByID)}, nil
}

type qualityAccumulator struct {
	count      int
	rttSum     uint64
	rttCount   int
	lossSum    uint64
	lossCount  int
	scoreSum   uint64
	scoreCount int
}

func (a *qualityAccumulator) add(record repo.NodePathHealth) {
	a.count++
	if record.ObservedRttMs != nil {
		a.rttSum += uint64(*record.ObservedRttMs)
		a.rttCount++
	}
	if record.PacketLossPpm != nil {
		a.lossSum += uint64(*record.PacketLossPpm)
		a.lossCount++
	}
	if record.PathScore != nil {
		a.scoreSum += uint64(*record.PathScore)
		a.scoreCount++
	}
}

func (a qualityAccumulator) dto() dto.OpsNetworkQualityCounter {
	out := dto.OpsNetworkQualityCounter{SampleCount: a.count}
	if a.rttCount > 0 {
		out.AvgRttMs = uint32(a.rttSum / uint64(a.rttCount))
	}
	if a.lossCount > 0 {
		out.AvgPacketLossPpm = uint32(a.lossSum / uint64(a.lossCount))
	}
	if a.scoreCount > 0 {
		out.AvgPathScore = uint32(a.scoreSum / uint64(a.scoreCount))
	}
	return out
}

func buildOpsNetworkQualitySummary(records []repo.NodePathHealth, networkNameByID map[string]string) dto.OpsNetworkQualitySummary {
	type networkAgg struct {
		cross            qualityAccumulator
		nonCross         qualityAccumulator
		pathCounts       map[string]int
		activePathCounts map[string]int
		pathDowngrades   uint64
		pathUpgrades     uint64
		pairs            map[string]*qualityAccumulator
		pairMeta         map[string]dto.OpsNetworkQualityCountryPair
	}
	grouped := make(map[string]*networkAgg)
	for _, record := range records {
		agg := grouped[record.NetworkID]
		if agg == nil {
			agg = &networkAgg{
				pathCounts:       make(map[string]int),
				activePathCounts: make(map[string]int),
				pairs:            make(map[string]*qualityAccumulator),
				pairMeta:         make(map[string]dto.OpsNetworkQualityCountryPair),
			}
			grouped[record.NetworkID] = agg
		}
		pathType := strings.TrimSpace(record.PathType)
		if pathType == "" {
			pathType = "unknown"
		}
		agg.pathCounts[pathType]++
		activePath := strings.TrimSpace(record.ActivePath)
		if activePath == "" {
			activePath = pathType
		}
		agg.activePathCounts[activePath]++
		agg.pathDowngrades += record.PathDowngrades
		agg.pathUpgrades += record.PathUpgrades
		isCross := record.CrossCountry != nil && *record.CrossCountry
		if isCross {
			agg.cross.add(record)
		} else {
			agg.nonCross.add(record)
		}
		pairKey := record.SourceCountryCode + "|" + record.RelayCountryCode + "|" + record.PeerCountryCode
		pair := agg.pairs[pairKey]
		if pair == nil {
			pair = &qualityAccumulator{}
			agg.pairs[pairKey] = pair
			agg.pairMeta[pairKey] = dto.OpsNetworkQualityCountryPair{
				SourceCountryCode: record.SourceCountryCode,
				RelayCountryCode:  record.RelayCountryCode,
				PeerCountryCode:   record.PeerCountryCode,
				CrossCountry:      isCross,
			}
		}
		pair.add(record)
	}
	out := make([]dto.OpsNetworkQualityNetworkSummary, 0, len(grouped))
	for networkID, agg := range grouped {
		pathTypes := make([]dto.OpsNetworkQualityPathType, 0, len(agg.pathCounts))
		for pathType, count := range agg.pathCounts {
			pathTypes = append(pathTypes, dto.OpsNetworkQualityPathType{PathType: pathType, Count: count})
		}
		sort.Slice(pathTypes, func(i, j int) bool {
			if pathTypes[i].Count != pathTypes[j].Count {
				return pathTypes[i].Count > pathTypes[j].Count
			}
			return pathTypes[i].PathType < pathTypes[j].PathType
		})
		activePaths := make([]dto.OpsNetworkQualityPathType, 0, len(agg.activePathCounts))
		for pathType, count := range agg.activePathCounts {
			activePaths = append(activePaths, dto.OpsNetworkQualityPathType{PathType: pathType, Count: count})
		}
		sort.Slice(activePaths, func(i, j int) bool {
			if activePaths[i].Count != activePaths[j].Count {
				return activePaths[i].Count > activePaths[j].Count
			}
			return activePaths[i].PathType < activePaths[j].PathType
		})
		pairs := make([]dto.OpsNetworkQualityCountryPair, 0, len(agg.pairs))
		for key, acc := range agg.pairs {
			item := agg.pairMeta[key]
			item.Counter = acc.dto()
			pairs = append(pairs, item)
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].Counter.SampleCount != pairs[j].Counter.SampleCount {
				return pairs[i].Counter.SampleCount > pairs[j].Counter.SampleCount
			}
			if pairs[i].SourceCountryCode != pairs[j].SourceCountryCode {
				return pairs[i].SourceCountryCode < pairs[j].SourceCountryCode
			}
			if pairs[i].RelayCountryCode != pairs[j].RelayCountryCode {
				return pairs[i].RelayCountryCode < pairs[j].RelayCountryCode
			}
			return pairs[i].PeerCountryCode < pairs[j].PeerCountryCode
		})
		out = append(out, dto.OpsNetworkQualityNetworkSummary{
			NetworkID:       networkID,
			NetworkName:     networkNameByID[networkID],
			HasCrossCountry: agg.cross.count > 0,
			CrossCountry:    agg.cross.dto(),
			NonCrossCountry: agg.nonCross.dto(),
			PathTypes:       pathTypes,
			ActivePaths:     activePaths,
			PathDowngrades:  agg.pathDowngrades,
			PathUpgrades:    agg.pathUpgrades,
			CountryPairs:    pairs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].NetworkID < out[j].NetworkID
	})
	return dto.OpsNetworkQualitySummary{Networks: out}
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
			AdminID:           admin.AdminID,
			UserID:            admin.UserID,
			LoginName:         admin.LoginName,
			Email:             userByID[admin.UserID],
			DisplayName:       admin.DisplayName,
			Phone:             admin.Phone,
			Title:             admin.Title,
			Department:        admin.Department,
			Status:            admin.Status,
			PasswordUpdatedAt: admin.PasswordUpdatedAt,
			LastLoginAt:       admin.LastLoginAt,
			LastLoginIP:       admin.LastLoginIP,
			FailedLoginCount:  admin.FailedLoginCount,
			LockedUntil:       admin.LockedUntil,
			UsingSeedPassword: s.adminUsesSeedPassword(admin),
			RoleIDs:           append([]string(nil), roleIDsByUser[admin.UserID]...),
			RoleCodes:         append([]string(nil), roleCodesByUser[admin.UserID]...),
			RoleNames:         append([]string(nil), roleNamesByUser[admin.UserID]...),
		})
	}
	return out, nil
}

func (s dbOpsService) opsSecurityWarnings(ctx context.Context) ([]string, error) {
	warnings := make([]string, 0, 2)
	if !s.state.cfg.Ops.DefaultAdmin.Enabled {
		return warnings, nil
	}
	admins, err := s.state.pg.ListAdminInfo(ctx)
	if err != nil {
		return nil, err
	}
	for _, admin := range admins {
		if s.adminUsesSeedPassword(admin) {
			warnings = append(warnings, "default admin password is still active for loginName="+admin.LoginName)
			break
		}
	}
	return warnings, nil
}

func (s dbOpsService) defaultAdminSeedStatus(ctx context.Context) (bool, bool, error) {
	cfg := s.state.cfg.Ops.DefaultAdmin
	if !cfg.Enabled || cfg.Email == "" {
		return false, false, nil
	}
	user, err := s.state.pg.GetUserByEmail(ctx, cfg.Email)
	if err != nil {
		if repo.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}
	admin, err := s.state.pg.GetAdminInfoByUserID(ctx, user.UserID)
	if err != nil {
		if repo.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}
	role, err := s.state.pg.GetRoleByCode(ctx, "ops-super-admin")
	if err != nil {
		if repo.IsNotFound(err) {
			return true, false, nil
		}
		return false, false, err
	}
	bindings, err := s.state.pg.ListUserRolesByUser(ctx, admin.UserID)
	if err != nil {
		return true, false, err
	}
	for _, binding := range bindings {
		if binding.RoleID == role.RoleID {
			return true, true, nil
		}
	}
	return true, false, nil
}

func (s dbOpsService) adminUsesSeedPassword(admin repo.AdminInfo) bool {
	cfg := s.state.cfg.Ops.DefaultAdmin
	if !cfg.Enabled || cfg.Password == "" {
		return false
	}
	if admin.LoginName != cfg.LoginName {
		return false
	}
	return admin.PasswordHash == util.HashPassword(cfg.Password)
}
