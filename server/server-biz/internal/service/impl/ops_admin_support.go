package impl

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

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
	rolesByUser := opsRolesByUser(roles, userRoles)
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
			RoleIDs:           append([]string(nil), rolesByUser.ids[admin.UserID]...),
			RoleCodes:         append([]string(nil), rolesByUser.codes[admin.UserID]...),
			RoleNames:         append([]string(nil), rolesByUser.names[admin.UserID]...),
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
