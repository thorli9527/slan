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

// UpsertAdminInfo 创建或更新管理员档案。
//
// 该方法只关心管理员基础身份信息，不负责角色和菜单绑定。
func (s dbOpsService) UpsertAdminInfo(req dto.UpsertAdminInfoRequest) (dto.OpsAdminInfo, error) {
	userID := strings.TrimSpace(req.UserID)
	loginName := strings.TrimSpace(strings.ToLower(req.LoginName))
	displayName := strings.TrimSpace(req.DisplayName)
	status := strings.TrimSpace(req.Status)
	if userID == "" || loginName == "" || displayName == "" {
		return dto.OpsAdminInfo{}, fmt.Errorf("%w: userId, loginName and displayName are required", ErrInvalidArgument)
	}
	if status == "" {
		status = "active"
	}

	ctx := context.Background()
	user, err := s.state.pg.GetUserByID(ctx, userID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.OpsAdminInfo{}, ErrNotFound
		}
		return dto.OpsAdminInfo{}, err
	}

	record := repo.AdminInfo{
		AdminID:           util.NewID("admin"),
		UserID:            userID,
		LoginName:         loginName,
		DisplayName:       displayName,
		Phone:             strings.TrimSpace(req.Phone),
		Title:             strings.TrimSpace(req.Title),
		Department:        strings.TrimSpace(req.Department),
		Status:            status,
		PasswordHash:      "",
		PasswordUpdatedAt: 0,
	}
	if existing, err := s.state.pg.GetAdminInfoByUserID(ctx, userID); err == nil {
		record.AdminID = existing.AdminID
		record.PasswordHash = existing.PasswordHash
		record.PasswordUpdatedAt = existing.PasswordUpdatedAt
		record.LastLoginAt = existing.LastLoginAt
		record.LastLoginIP = existing.LastLoginIP
		record.FailedLoginCount = existing.FailedLoginCount
		record.LockedUntil = existing.LockedUntil
	} else if !repo.IsNotFound(err) {
		return dto.OpsAdminInfo{}, err
	}
	if req.Password != "" {
		if len(req.Password) < 8 {
			return dto.OpsAdminInfo{}, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidArgument)
		}
		record.PasswordHash = util.HashPassword(req.Password)
		record.PasswordUpdatedAt = time.Now().UnixMilli()
	}
	if record.PasswordHash == "" {
		return dto.OpsAdminInfo{}, fmt.Errorf("%w: password is required for new admin", ErrInvalidArgument)
	}
	if err := s.state.pg.UpsertAdminInfo(ctx, record); err != nil {
		return dto.OpsAdminInfo{}, err
	}

	return dto.OpsAdminInfo{
		AdminID:           record.AdminID,
		UserID:            userID,
		LoginName:         record.LoginName,
		Email:             user.Email,
		DisplayName:       record.DisplayName,
		Phone:             record.Phone,
		Title:             record.Title,
		Department:        record.Department,
		Status:            record.Status,
		PasswordUpdatedAt: record.PasswordUpdatedAt,
		LastLoginAt:       record.LastLoginAt,
		LastLoginIP:       record.LastLoginIP,
		FailedLoginCount:  record.FailedLoginCount,
		LockedUntil:       record.LockedUntil,
	}, nil
}

// ChangeAdminPassword 修改管理员登录密码。
func (s dbOpsService) ChangeAdminPassword(adminID string, req dto.ChangeAdminPasswordRequest) error {
	adminID = strings.TrimSpace(adminID)
	if adminID == "" {
		return fmt.Errorf("%w: adminId is required", ErrInvalidArgument)
	}
	if len(req.Password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidArgument)
	}

	ctx := context.Background()
	admin, err := s.state.pg.GetAdminInfoByID(ctx, adminID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return s.state.pg.UpdateAdminPasswordAudit(
		ctx,
		admin.AdminID,
		util.HashPassword(req.Password),
		time.Now().UnixMilli(),
	)
}

// UnlockAdmin 清除管理员锁定状态和失败计数。
func (s dbOpsService) ChangeOwnAdminPassword(adminID string, req dto.ChangeAdminPasswordRequest) error {
	return s.ChangeAdminPassword(adminID, req)
}

func (s dbOpsService) UnlockAdmin(adminID string) error {
	adminID = strings.TrimSpace(adminID)
	if adminID == "" {
		return fmt.Errorf("%w: adminId is required", ErrInvalidArgument)
	}

	ctx := context.Background()
	admin, err := s.state.pg.GetAdminInfoByID(ctx, adminID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return s.state.pg.RecordAdminLoginFailure(ctx, admin.AdminID, 0, 0)
}
