package impl

import (
	"context"
	"strings"

	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

var builtinOpsMenus = []repo.Menu{
	{MenuCode: "ops.overview", MenuName: "Overview", Path: "/overview", Sort: 10, Status: "active"},
	{MenuCode: "ops.users", MenuName: "Users", Path: "/users", Sort: 20, Status: "active"},
	{MenuCode: "ops.devices", MenuName: "Devices", Path: "/devices", Sort: 30, Status: "active"},
	{MenuCode: "ops.admins", MenuName: "Admins", Path: "/admins", Sort: 40, Status: "active"},
	{MenuCode: "ops.roles", MenuName: "Roles", Path: "/roles", Sort: 50, Status: "active"},
	{MenuCode: "ops.menus", MenuName: "Menus", Path: "/menus", Sort: 60, Status: "active"},
	{MenuCode: "ops.relays", MenuName: "Relays", Path: "/relays", Sort: 70, Status: "active"},
	{MenuCode: "ops.settings", MenuName: "Settings", Path: "/settings", Sort: 80, Status: "active"},
}

type builtinOpsRoleSeed struct {
	roleCode  string
	roleName  string
	desc      string
	menuCodes []string
}

var builtinOpsRoles = []builtinOpsRoleSeed{
	{
		roleCode: "ops-super-admin",
		roleName: "Ops Super Admin",
		desc:     "Full access to every ops route.",
		menuCodes: []string{
			"ops.overview",
			"ops.users",
			"ops.devices",
			"ops.admins",
			"ops.roles",
			"ops.menus",
			"ops.relays",
			"ops.settings",
		},
	},
	{
		roleCode: "ops-observer",
		roleName: "Ops Observer",
		desc:     "Read-only access to overview, users, devices, and relays.",
		menuCodes: []string{
			"ops.overview",
			"ops.users",
			"ops.devices",
			"ops.relays",
		},
	},
}

func (s *dbState) seedBuiltinOpsRBAC(ctx context.Context) error {
	menuIDByCode := make(map[string]string, len(builtinOpsMenus))
	for _, item := range builtinOpsMenus {
		menu, err := s.statefulGetOrCreateMenu(ctx, item)
		if err != nil {
			return err
		}
		menuIDByCode[menu.MenuCode] = menu.MenuID
	}
	for _, item := range builtinOpsRoles {
		role, err := s.statefulGetOrCreateRole(ctx, item)
		if err != nil {
			return err
		}
		menuIDs := make([]string, 0, len(item.menuCodes))
		for _, code := range item.menuCodes {
			if menuID, ok := menuIDByCode[code]; ok {
				menuIDs = append(menuIDs, menuID)
			}
		}
		if err := s.pg.ReplaceRoleMenus(ctx, role.RoleID, menuIDs); err != nil {
			return err
		}
	}
	return nil
}

func (s *dbState) seedDefaultAdmin(ctx context.Context) error {
	cfg := s.cfg.Ops.DefaultAdmin
	if !cfg.Enabled {
		return nil
	}
	email := util.NormalizeEmail(cfg.Email)
	loginName := strings.TrimSpace(strings.ToLower(cfg.LoginName))
	displayName := strings.TrimSpace(cfg.DisplayName)
	password := cfg.Password
	if email == "" || loginName == "" || displayName == "" || password == "" {
		return nil
	}

	user, err := s.pg.GetUserByEmail(ctx, email)
	if err != nil {
		if !repo.IsNotFound(err) {
			return err
		}
		user = repo.User{
			UserID:       util.NewID("user"),
			Email:        email,
			PasswordHash: util.HashPassword(password),
		}
		if err := s.pg.CreateUser(ctx, user); err != nil {
			if existing, getErr := s.pg.GetUserByEmail(ctx, email); getErr == nil {
				user = existing
			} else {
				return err
			}
		}
	}

	admin, err := s.pg.GetAdminInfoByUserID(ctx, user.UserID)
	if err != nil {
		if !repo.IsNotFound(err) {
			return err
		}
		admin = repo.AdminInfo{
			AdminID:           util.NewID("admin"),
			UserID:            user.UserID,
			LoginName:         loginName,
			PasswordHash:      util.HashPassword(password),
			PasswordUpdatedAt: 0,
			DisplayName:       displayName,
			Status:            "active",
		}
		if err := s.pg.UpsertAdminInfo(ctx, admin); err != nil {
			return err
		}
	}

	role, err := s.pg.GetRoleByCode(ctx, "ops-super-admin")
	if err != nil {
		return err
	}
	bindings, err := s.pg.ListUserRolesByUser(ctx, user.UserID)
	if err != nil {
		return err
	}
	roleIDs := make([]string, 0, len(bindings)+1)
	exists := false
	for _, binding := range bindings {
		roleIDs = append(roleIDs, binding.RoleID)
		if binding.RoleID == role.RoleID {
			exists = true
		}
	}
	if !exists {
		roleIDs = append(roleIDs, role.RoleID)
		if err := s.pg.ReplaceUserRoles(ctx, user.UserID, util.DedupeTrimmed(roleIDs)); err != nil {
			return err
		}
	}
	return nil
}

func (s *dbState) statefulGetOrCreateMenu(ctx context.Context, item repo.Menu) (repo.Menu, error) {
	menu, err := s.pg.GetMenuByCode(ctx, item.MenuCode)
	if err == nil {
		return menu, nil
	}
	if !repo.IsNotFound(err) {
		return repo.Menu{}, err
	}
	item.MenuID = util.NewID("menu")
	if err := s.pg.InsertMenu(ctx, item); err != nil {
		if !repo.IsNotFound(err) {
			if menu, getErr := s.pg.GetMenuByCode(ctx, item.MenuCode); getErr == nil {
				return menu, nil
			}
		}
		if menu, getErr := s.pg.GetMenuByCode(ctx, item.MenuCode); getErr == nil {
			return menu, nil
		}
		return repo.Menu{}, err
	}
	return item, nil
}

func (s *dbState) statefulGetOrCreateRole(ctx context.Context, item builtinOpsRoleSeed) (repo.Role, error) {
	role, err := s.pg.GetRoleByCode(ctx, item.roleCode)
	if err == nil {
		return role, nil
	}
	if !repo.IsNotFound(err) {
		return repo.Role{}, err
	}
	role = repo.Role{
		RoleID:      util.NewID("role"),
		RoleCode:    item.roleCode,
		RoleName:    item.roleName,
		Description: item.desc,
		Builtin:     true,
	}
	if err := s.pg.InsertRole(ctx, role); err != nil {
		if existing, getErr := s.pg.GetRoleByCode(ctx, item.roleCode); getErr == nil {
			return existing, nil
		}
		return repo.Role{}, err
	}
	return role, nil
}
