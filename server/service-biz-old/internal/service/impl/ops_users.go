package impl

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

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
	counts := opsUserCounts(devices, nodes)
	rolesByUser := opsRolesByUser(roles, userRoles)
	userPlanOverrides := s.opsUserPlanOverrides(ctx, users)
	out := make([]dto.OpsUser, 0, len(users))
	for _, user := range users {
		item := dto.OpsUser{
			UserID:      user.UserID,
			Email:       user.Email,
			DeviceCount: counts.deviceCountByUser[user.UserID],
			NodeCount:   counts.nodeCountByUser[user.UserID],
			RoleIDs:     append([]string(nil), rolesByUser.ids[user.UserID]...),
			RoleCodes:   append([]string(nil), rolesByUser.codes[user.UserID]...),
			RoleNames:   append([]string(nil), rolesByUser.names[user.UserID]...),
		}
		if override, ok := userPlanOverrides[user.UserID]; ok {
			copy := override
			item.PlanOverride = &copy
		}
		out = append(out, item)
	}
	return out, nil
}

type opsUserCountMaps struct {
	deviceCountByUser map[string]int
	nodeCountByUser   map[string]int
}

func opsUserCounts(devices []repo.Device, nodes []repo.Node) opsUserCountMaps {
	out := opsUserCountMaps{
		deviceCountByUser: make(map[string]int, len(devices)),
		nodeCountByUser:   make(map[string]int, len(nodes)),
	}
	for _, device := range devices {
		out.deviceCountByUser[device.UserID]++
	}
	for _, node := range nodes {
		out.nodeCountByUser[node.UserID]++
	}
	return out
}

type opsUserRoleMaps struct {
	ids   map[string][]string
	codes map[string][]string
	names map[string][]string
}

func opsRolesByUser(roles []repo.Role, bindings []repo.UserRole) opsUserRoleMaps {
	roleByID := make(map[string]repo.Role, len(roles))
	for _, role := range roles {
		roleByID[role.RoleID] = role
	}
	out := opsUserRoleMaps{
		ids:   make(map[string][]string),
		codes: make(map[string][]string),
		names: make(map[string][]string),
	}
	for _, binding := range bindings {
		out.ids[binding.UserID] = append(out.ids[binding.UserID], binding.RoleID)
		if role, ok := roleByID[binding.RoleID]; ok {
			out.codes[binding.UserID] = append(out.codes[binding.UserID], role.RoleCode)
			out.names[binding.UserID] = append(out.names[binding.UserID], role.RoleName)
		}
	}
	return out
}

func (s dbOpsService) opsUserPlanOverrides(ctx context.Context, users []repo.User) map[string]dto.PlanConfig {
	out := make(map[string]dto.PlanConfig)
	for _, user := range users {
		if override, err := s.state.pg.GetUserPlanOverride(ctx, user.UserID); err == nil {
			out[user.UserID] = dto.PlanConfig{
				MaxActiveDevices: override.MaxActiveDevices,
				RelayIngressKbps: override.RelayIngressKbps,
				RelayEgressKbps:  override.RelayEgressKbps,
				UDPIngressKbps:   override.UDPIngressKbps,
				UDPEgressKbps:    override.UDPEgressKbps,
			}
		}
	}
	return out
}
