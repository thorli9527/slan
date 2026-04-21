package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

// ListRoles 返回角色及其绑定菜单的视图。
func (s dbOpsService) ListRoles() ([]dto.OpsRole, error) {
	ctx := context.Background()
	roles, err := s.state.pg.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	menus, err := s.state.pg.ListMenus(ctx)
	if err != nil {
		return nil, err
	}
	roleMenus, err := s.state.pg.ListRoleMenus(ctx)
	if err != nil {
		return nil, err
	}

	menuByID := make(map[string]repo.Menu, len(menus))
	for _, menu := range menus {
		menuByID[menu.MenuID] = menu
	}
	menuIDsByRole := make(map[string][]string)
	menuCodesByRole := make(map[string][]string)
	menuNamesByRole := make(map[string][]string)
	for _, binding := range roleMenus {
		menuIDsByRole[binding.RoleID] = append(menuIDsByRole[binding.RoleID], binding.MenuID)
		if menu, ok := menuByID[binding.MenuID]; ok {
			menuCodesByRole[binding.RoleID] = append(menuCodesByRole[binding.RoleID], menu.MenuCode)
			menuNamesByRole[binding.RoleID] = append(menuNamesByRole[binding.RoleID], menu.MenuName)
		}
	}

	out := make([]dto.OpsRole, 0, len(roles))
	for _, role := range roles {
		out = append(out, dto.OpsRole{
			RoleID:      role.RoleID,
			RoleCode:    role.RoleCode,
			RoleName:    role.RoleName,
			Description: role.Description,
			Builtin:     role.Builtin,
			MenuIDs:     append([]string(nil), menuIDsByRole[role.RoleID]...),
			MenuCodes:   append([]string(nil), menuCodesByRole[role.RoleID]...),
			MenuNames:   append([]string(nil), menuNamesByRole[role.RoleID]...),
		})
	}
	return out, nil
}

// CreateRole 创建一个新的自定义角色。
func (s dbOpsService) CreateRole(req dto.CreateRoleRequest) (dto.OpsRole, error) {
	roleCode := strings.TrimSpace(req.RoleCode)
	roleName := strings.TrimSpace(req.RoleName)
	if roleCode == "" || roleName == "" {
		return dto.OpsRole{}, fmt.Errorf("%w: roleCode and roleName are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	if _, err := s.state.pg.GetRoleByCode(ctx, roleCode); err == nil {
		return dto.OpsRole{}, fmt.Errorf("%w: role code already exists", ErrConflict)
	} else if !repo.IsNotFound(err) {
		return dto.OpsRole{}, err
	}

	record := repo.Role{
		RoleID:      util.NewID("role"),
		RoleCode:    roleCode,
		RoleName:    roleName,
		Description: strings.TrimSpace(req.Description),
		Builtin:     false,
	}
	if err := s.state.pg.InsertRole(ctx, record); err != nil {
		return dto.OpsRole{}, err
	}

	return dto.OpsRole{
		RoleID:      record.RoleID,
		RoleCode:    record.RoleCode,
		RoleName:    record.RoleName,
		Description: record.Description,
		Builtin:     record.Builtin,
	}, nil
}

// AssignUserRoles 替换目标用户的全部角色绑定。
func (s dbOpsService) AssignUserRoles(userID string, req dto.AssignUserRolesRequest) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("%w: userId is required", ErrInvalidArgument)
	}

	ctx := context.Background()
	if _, err := s.state.pg.GetUserByID(ctx, userID); err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}

	roleIDs := util.DedupeTrimmed(req.RoleIDs)
	for _, roleID := range roleIDs {
		if _, err := s.state.pg.GetRoleByID(ctx, roleID); err != nil {
			if repo.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
	}
	return s.state.pg.ReplaceUserRoles(ctx, userID, roleIDs)
}

// ListMenus 返回菜单树的平铺视图。
func (s dbOpsService) ListMenus() ([]dto.OpsMenu, error) {
	menus, err := s.state.pg.ListMenus(context.Background())
	if err != nil {
		return nil, err
	}

	out := make([]dto.OpsMenu, 0, len(menus))
	for _, menu := range menus {
		out = append(out, dto.OpsMenu{
			MenuID:   menu.MenuID,
			MenuCode: menu.MenuCode,
			MenuName: menu.MenuName,
			Path:     menu.Path,
			ParentID: menu.ParentID,
			Sort:     menu.Sort,
			Status:   menu.Status,
		})
	}
	return out, nil
}

// CreateMenu 创建一个新的功能菜单节点。
func (s dbOpsService) CreateMenu(req dto.CreateMenuRequest) (dto.OpsMenu, error) {
	menuCode := strings.TrimSpace(req.MenuCode)
	menuName := strings.TrimSpace(req.MenuName)
	if menuCode == "" || menuName == "" {
		return dto.OpsMenu{}, fmt.Errorf("%w: menuCode and menuName are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	if _, err := s.state.pg.GetMenuByCode(ctx, menuCode); err == nil {
		return dto.OpsMenu{}, fmt.Errorf("%w: menu code already exists", ErrConflict)
	} else if !repo.IsNotFound(err) {
		return dto.OpsMenu{}, err
	}

	parentID := strings.TrimSpace(req.ParentID)
	if parentID != "" {
		if _, err := s.state.pg.GetMenuByID(ctx, parentID); err != nil {
			if repo.IsNotFound(err) {
				return dto.OpsMenu{}, ErrNotFound
			}
			return dto.OpsMenu{}, err
		}
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}

	record := repo.Menu{
		MenuID:   util.NewID("menu"),
		MenuCode: menuCode,
		MenuName: menuName,
		Path:     strings.TrimSpace(req.Path),
		ParentID: parentID,
		Sort:     req.Sort,
		Status:   status,
	}
	if err := s.state.pg.InsertMenu(ctx, record); err != nil {
		return dto.OpsMenu{}, err
	}

	return dto.OpsMenu{
		MenuID:   record.MenuID,
		MenuCode: record.MenuCode,
		MenuName: record.MenuName,
		Path:     record.Path,
		ParentID: record.ParentID,
		Sort:     record.Sort,
		Status:   record.Status,
	}, nil
}

// AssignRoleMenus 替换目标角色绑定的全部菜单。
func (s dbOpsService) AssignRoleMenus(roleID string, req dto.AssignRoleMenusRequest) error {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return fmt.Errorf("%w: roleId is required", ErrInvalidArgument)
	}

	ctx := context.Background()
	if _, err := s.state.pg.GetRoleByID(ctx, roleID); err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}

	menuIDs := util.DedupeTrimmed(req.MenuIDs)
	for _, menuID := range menuIDs {
		if _, err := s.state.pg.GetMenuByID(ctx, menuID); err != nil {
			if repo.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
	}
	return s.state.pg.ReplaceRoleMenus(ctx, roleID, menuIDs)
}
