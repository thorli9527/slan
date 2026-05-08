package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s dbOpsService) LogoutAdmin(accessToken string) error {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil
	}
	return s.state.tokens.DeleteOpsAccessToken(context.Background(), accessToken)
}

// LoginAdmin 校验管理员登录名和密码，并签发 ops token。
func (s dbOpsService) LoginAdmin(req dto.OpsLoginRequest, remoteIP string) (dto.OpsLoginResponse, error) {
	loginName := strings.TrimSpace(strings.ToLower(req.LoginName))
	if loginName == "" || req.Password == "" {
		return dto.OpsLoginResponse{}, fmt.Errorf("%w: loginName and password are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	admin, err := s.state.pg.GetAdminInfoByLoginName(ctx, loginName)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.OpsLoginResponse{}, ErrUnauthorized
		}
		return dto.OpsLoginResponse{}, err
	}
	if admin.Status != "active" {
		return dto.OpsLoginResponse{}, ErrForbidden
	}
	now := time.Now().UnixMilli()
	if admin.LockedUntil > now {
		return dto.OpsLoginResponse{}, ErrForbidden
	}
	if admin.PasswordHash == "" || admin.PasswordHash != util.HashPassword(req.Password) {
		failedCount := admin.FailedLoginCount + 1
		lockedUntil := int64(0)
		if failedCount >= 5 {
			lockedUntil = time.Now().Add(15 * time.Minute).UnixMilli()
		}
		if updateErr := s.state.pg.RecordAdminLoginFailure(ctx, admin.AdminID, failedCount, lockedUntil); updateErr != nil {
			return dto.OpsLoginResponse{}, updateErr
		}
		return dto.OpsLoginResponse{}, ErrUnauthorized
	}
	userRoles, err := s.state.pg.ListUserRolesByUser(ctx, admin.UserID)
	if err != nil {
		return dto.OpsLoginResponse{}, err
	}
	if len(userRoles) == 0 {
		return dto.OpsLoginResponse{}, ErrForbidden
	}

	token := util.OpaqueToken("ops-access", admin.AdminID)
	if err := s.state.tokens.StoreOpsAccessToken(ctx, token, admin.AdminID, 8*time.Hour); err != nil {
		return dto.OpsLoginResponse{}, err
	}
	if err := s.state.pg.UpdateAdminLoginAudit(ctx, admin.AdminID, time.Now().UnixMilli(), strings.TrimSpace(remoteIP)); err != nil {
		return dto.OpsLoginResponse{}, err
	}
	return dto.OpsLoginResponse{
		AdminID:     admin.AdminID,
		UserID:      admin.UserID,
		LoginName:   admin.LoginName,
		DisplayName: admin.DisplayName,
		AccessToken: token,
		ExpiresIn:   int64((8 * time.Hour).Seconds()),
	}, nil
}

// AuthenticateAdminToken 校验管理员登录后的 ops access token。
func (s dbOpsService) AuthenticateAdminToken(accessToken string) (string, error) {
	ctx := context.Background()
	adminID, err := s.state.tokens.AuthenticateOpsAccessToken(ctx, accessToken)
	if err != nil {
		return "", ErrUnauthorized
	}
	admin, err := s.state.pg.GetAdminInfoByID(ctx, adminID)
	if err != nil {
		return "", ErrUnauthorized
	}
	if admin.Status != "active" {
		return "", ErrForbidden
	}
	userRoles, err := s.state.pg.ListUserRolesByUser(ctx, admin.UserID)
	if err != nil {
		return "", err
	}
	if len(userRoles) == 0 {
		return "", ErrForbidden
	}
	return adminID, nil
}

// AuthorizeAdminMenu 校验管理员是否具备指定菜单码的访问权限。
func (s dbOpsService) AuthorizeAdminMenu(adminID string, menuCode string) error {
	adminID = strings.TrimSpace(adminID)
	menuCode = strings.TrimSpace(menuCode)
	if adminID == "" || menuCode == "" {
		return ErrForbidden
	}

	ctx := context.Background()
	admin, err := s.state.pg.GetAdminInfoByID(ctx, adminID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrUnauthorized
		}
		return err
	}
	menu, err := s.state.pg.GetMenuByCode(ctx, menuCode)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrForbidden
		}
		return err
	}
	userRoles, err := s.state.pg.ListUserRolesByUser(ctx, admin.UserID)
	if err != nil {
		return err
	}
	if len(userRoles) == 0 {
		return ErrForbidden
	}
	menuBindings, err := s.state.pg.ListRoleMenus(ctx)
	if err != nil {
		return err
	}
	roleIDs := make(map[string]struct{}, len(userRoles))
	for _, binding := range userRoles {
		roleIDs[binding.RoleID] = struct{}{}
	}
	for _, binding := range menuBindings {
		if binding.MenuID != menu.MenuID {
			continue
		}
		if _, ok := roleIDs[binding.RoleID]; ok {
			return nil
		}
	}
	return ErrForbidden
}
