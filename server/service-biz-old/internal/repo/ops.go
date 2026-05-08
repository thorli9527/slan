package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// AdminInfo 是管理员扩展资料的持久化模型。
type AdminInfo struct {
	// AdminID 是管理员资料记录唯一标识。
	AdminID string `gorm:"column:admin_id;primaryKey"`
	// UserID 是绑定的业务用户 ID。
	UserID string `gorm:"column:user_id;uniqueIndex;not null"`
	// LoginName 是管理员登录名。
	LoginName string `gorm:"column:login_name;uniqueIndex;not null"`
	// PasswordHash 是管理员登录密码哈希。
	PasswordHash string `gorm:"column:password_hash;not null;default:''"`
	// PasswordUpdatedAt 是最近一次密码更新时间戳，单位毫秒。
	PasswordUpdatedAt int64 `gorm:"column:password_updated_at;not null;default:0"`
	// DisplayName 是管理员显示名称。
	DisplayName string `gorm:"column:display_name;not null"`
	// Phone 是联系电话。
	Phone string `gorm:"column:phone;not null;default:''"`
	// Title 是岗位或职务。
	Title string `gorm:"column:title;not null;default:''"`
	// Department 是所属部门。
	Department string `gorm:"column:department;not null;default:''"`
	// Status 是管理员状态。
	Status string `gorm:"column:status;not null;default:'active'"`
	// LastLoginAt 是最近一次成功登录时间戳，单位毫秒。
	LastLoginAt int64 `gorm:"column:last_login_at;not null;default:0"`
	// LastLoginIP 是最近一次成功登录来源 IP。
	LastLoginIP string `gorm:"column:last_login_ip;not null;default:''"`
	// FailedLoginCount 是连续登录失败次数。
	FailedLoginCount int `gorm:"column:failed_login_count;not null;default:0"`
	// LockedUntil 是账号锁定截止时间戳，单位毫秒；0 表示未锁定。
	LockedUntil int64 `gorm:"column:locked_until;not null;default:0"`
}

func (AdminInfo) TableName() string { return "admin_info" }

// Role 是 RBAC 角色的持久化模型。
type Role struct {
	// RoleID 是角色唯一标识。
	RoleID string `gorm:"column:role_id;primaryKey"`
	// RoleCode 是角色稳定编码。
	RoleCode string `gorm:"column:role_code;uniqueIndex;not null"`
	// RoleName 是角色名称。
	RoleName string `gorm:"column:role_name;not null"`
	// Description 是角色说明。
	Description string `gorm:"column:description;not null;default:''"`
	// Builtin 表示是否为系统内置角色。
	Builtin bool `gorm:"column:builtin;not null;default:false"`
}

func (Role) TableName() string { return "roles" }

// UserRole 是用户与角色之间的绑定关系模型。
type UserRole struct {
	// BindingID 是绑定关系唯一标识。
	BindingID string `gorm:"column:binding_id;primaryKey"`
	// UserID 是用户 ID。
	UserID string `gorm:"column:user_id;index;not null;uniqueIndex:idx_user_role"`
	// RoleID 是角色 ID。
	RoleID string `gorm:"column:role_id;index;not null;uniqueIndex:idx_user_role"`
}

func (UserRole) TableName() string { return "user_roles" }

type PlanConfig struct {
	ConfigID         string `gorm:"column:config_id;primaryKey"`
	MaxActiveDevices int    `gorm:"column:max_active_devices;not null;default:5"`
	RelayIngressKbps int    `gorm:"column:relay_ingress_kbps;not null;default:512"`
	RelayEgressKbps  int    `gorm:"column:relay_egress_kbps;not null;default:512"`
	UDPIngressKbps   int    `gorm:"column:udp_ingress_kbps;not null;default:0"`
	UDPEgressKbps    int    `gorm:"column:udp_egress_kbps;not null;default:0"`
	UpdatedAt        int64  `gorm:"column:updated_at;not null;default:0"`
}

func (PlanConfig) TableName() string { return "plan_configs" }

type UserPlanOverride struct {
	UserID           string `gorm:"column:user_id;primaryKey"`
	MaxActiveDevices int    `gorm:"column:max_active_devices;not null;default:0"`
	RelayIngressKbps int    `gorm:"column:relay_ingress_kbps;not null;default:0"`
	RelayEgressKbps  int    `gorm:"column:relay_egress_kbps;not null;default:0"`
	UDPIngressKbps   int    `gorm:"column:udp_ingress_kbps;not null;default:0"`
	UDPEgressKbps    int    `gorm:"column:udp_egress_kbps;not null;default:0"`
	UpdatedAt        int64  `gorm:"column:updated_at;not null;default:0"`
}

func (UserPlanOverride) TableName() string { return "user_plan_overrides" }

// Menu 是运营平台功能菜单的持久化模型。
type Menu struct {
	// MenuID 是菜单唯一标识。
	MenuID string `gorm:"column:menu_id;primaryKey"`
	// MenuCode 是稳定菜单编码。
	MenuCode string `gorm:"column:menu_code;uniqueIndex;not null"`
	// MenuName 是菜单名称。
	MenuName string `gorm:"column:menu_name;not null"`
	// Path 是前端路由或功能入口。
	Path string `gorm:"column:path;not null;default:''"`
	// ParentID 是父菜单 ID。
	ParentID string `gorm:"column:parent_id;index;not null;default:''"`
	// Sort 是排序权重。
	Sort int `gorm:"column:sort;not null;default:0"`
	// Status 是菜单状态。
	Status string `gorm:"column:status;not null;default:'active'"`
}

func (Menu) TableName() string { return "menus" }

// RoleMenu 是角色与菜单之间的绑定关系模型。
type RoleMenu struct {
	// BindingID 是绑定关系唯一标识。
	BindingID string `gorm:"column:binding_id;primaryKey"`
	// RoleID 是角色 ID。
	RoleID string `gorm:"column:role_id;index;not null;uniqueIndex:idx_role_menu"`
	// MenuID 是菜单 ID。
	MenuID string `gorm:"column:menu_id;index;not null;uniqueIndex:idx_role_menu"`
}

func (RoleMenu) TableName() string { return "role_menus" }

func (r *PostgresRepository) GetAdminInfoByUserID(ctx context.Context, userID string) (AdminInfo, error) {
	var record AdminInfo
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) GetAdminInfoByID(ctx context.Context, adminID string) (AdminInfo, error) {
	var record AdminInfo
	err := r.db.WithContext(ctx).Where("admin_id = ?", adminID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) GetAdminInfoByLoginName(ctx context.Context, loginName string) (AdminInfo, error) {
	var record AdminInfo
	err := r.db.WithContext(ctx).Where("login_name = ?", loginName).First(&record).Error
	return record, err
}

func (r *PostgresRepository) UpsertAdminInfo(ctx context.Context, record AdminInfo) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", record.UserID).
		Assign(map[string]any{
			"login_name":          record.LoginName,
			"password_hash":       record.PasswordHash,
			"password_updated_at": record.PasswordUpdatedAt,
			"display_name":        record.DisplayName,
			"phone":               record.Phone,
			"title":               record.Title,
			"department":          record.Department,
			"status":              record.Status,
			"last_login_at":       record.LastLoginAt,
			"last_login_ip":       record.LastLoginIP,
			"failed_login_count":  record.FailedLoginCount,
			"locked_until":        record.LockedUntil,
		}).
		FirstOrCreate(&record).Error
}

func (r *PostgresRepository) UpdateAdminLoginAudit(ctx context.Context, adminID string, lastLoginAt int64, lastLoginIP string) error {
	return r.db.WithContext(ctx).
		Model(&AdminInfo{}).
		Where("admin_id = ?", adminID).
		Updates(map[string]any{
			"last_login_at":      lastLoginAt,
			"last_login_ip":      lastLoginIP,
			"failed_login_count": 0,
			"locked_until":       0,
		}).Error
}

func (r *PostgresRepository) UpdateAdminPasswordAudit(ctx context.Context, adminID string, passwordHash string, passwordUpdatedAt int64) error {
	return r.db.WithContext(ctx).
		Model(&AdminInfo{}).
		Where("admin_id = ?", adminID).
		Updates(map[string]any{
			"password_hash":       passwordHash,
			"password_updated_at": passwordUpdatedAt,
		}).Error
}

func (r *PostgresRepository) RecordAdminLoginFailure(ctx context.Context, adminID string, failedLoginCount int, lockedUntil int64) error {
	return r.db.WithContext(ctx).
		Model(&AdminInfo{}).
		Where("admin_id = ?", adminID).
		Updates(map[string]any{
			"failed_login_count": failedLoginCount,
			"locked_until":       lockedUntil,
		}).Error
}

func (r *PostgresRepository) ListAdminInfo(ctx context.Context) ([]AdminInfo, error) {
	var out []AdminInfo
	err := r.db.WithContext(ctx).Order("admin_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) InsertRole(ctx context.Context, record Role) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) GetRoleByID(ctx context.Context, roleID string) (Role, error) {
	var record Role
	err := r.db.WithContext(ctx).Where("role_id = ?", roleID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) GetRoleByCode(ctx context.Context, roleCode string) (Role, error) {
	var record Role
	err := r.db.WithContext(ctx).Where("role_code = ?", roleCode).First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListRoles(ctx context.Context) ([]Role, error) {
	var out []Role
	err := r.db.WithContext(ctx).Order("role_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ReplaceUserRoles(ctx context.Context, userID string, roleIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		db := tx
		if err := db.Where("user_id = ?", userID).Delete(&UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		records := make([]UserRole, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			records = append(records, UserRole{
				BindingID: fmt.Sprintf("user-role:%s:%s", userID, roleID),
				UserID:    userID,
				RoleID:    roleID,
			})
		}
		return db.Create(&records).Error
	})
}

func (r *PostgresRepository) ListUserRoles(ctx context.Context) ([]UserRole, error) {
	var out []UserRole
	err := r.db.WithContext(ctx).Order("binding_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListUserRolesByUser(ctx context.Context, userID string) ([]UserRole, error) {
	var out []UserRole
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("binding_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) GetPlanConfig(ctx context.Context, configID string) (PlanConfig, error) {
	var record PlanConfig
	err := r.db.WithContext(ctx).Where("config_id = ?", configID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) UpsertPlanConfig(ctx context.Context, record PlanConfig) error {
	return r.db.WithContext(ctx).
		Where("config_id = ?", record.ConfigID).
		Assign(map[string]any{
			"max_active_devices": record.MaxActiveDevices,
			"relay_ingress_kbps": record.RelayIngressKbps,
			"relay_egress_kbps":  record.RelayEgressKbps,
			"udp_ingress_kbps":   record.UDPIngressKbps,
			"udp_egress_kbps":    record.UDPEgressKbps,
			"updated_at":         record.UpdatedAt,
		}).
		FirstOrCreate(&record).Error
}

func (r *PostgresRepository) GetUserPlanOverride(ctx context.Context, userID string) (UserPlanOverride, error) {
	var record UserPlanOverride
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) UpsertUserPlanOverride(ctx context.Context, record UserPlanOverride) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", record.UserID).
		Assign(map[string]any{
			"max_active_devices": record.MaxActiveDevices,
			"relay_ingress_kbps": record.RelayIngressKbps,
			"relay_egress_kbps":  record.RelayEgressKbps,
			"udp_ingress_kbps":   record.UDPIngressKbps,
			"udp_egress_kbps":    record.UDPEgressKbps,
			"updated_at":         record.UpdatedAt,
		}).
		FirstOrCreate(&record).Error
}

func (r *PostgresRepository) DeleteUserPlanOverride(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&UserPlanOverride{}).Error
}

func (r *PostgresRepository) InsertMenu(ctx context.Context, record Menu) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) GetMenuByID(ctx context.Context, menuID string) (Menu, error) {
	var record Menu
	err := r.db.WithContext(ctx).Where("menu_id = ?", menuID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) GetMenuByCode(ctx context.Context, menuCode string) (Menu, error) {
	var record Menu
	err := r.db.WithContext(ctx).Where("menu_code = ?", menuCode).First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListMenus(ctx context.Context) ([]Menu, error) {
	var out []Menu
	err := r.db.WithContext(ctx).Order("sort asc, menu_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ReplaceRoleMenus(ctx context.Context, roleID string, menuIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		db := tx
		if err := db.Where("role_id = ?", roleID).Delete(&RoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuIDs) == 0 {
			return nil
		}
		records := make([]RoleMenu, 0, len(menuIDs))
		for _, menuID := range menuIDs {
			records = append(records, RoleMenu{
				BindingID: fmt.Sprintf("role-menu:%s:%s", roleID, menuID),
				RoleID:    roleID,
				MenuID:    menuID,
			})
		}
		return db.Create(&records).Error
	})
}

func (r *PostgresRepository) ListRoleMenus(ctx context.Context) ([]RoleMenu, error) {
	var out []RoleMenu
	err := r.db.WithContext(ctx).Order("binding_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListRoleMenusByRole(ctx context.Context, roleID string) ([]RoleMenu, error) {
	var out []RoleMenu
	err := r.db.WithContext(ctx).Where("role_id = ?", roleID).Order("binding_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) CountActiveAttachmentsByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("subnet_attachments").
		Joins("join network_members on network_members.network_id = subnet_attachments.network_id and network_members.device_id = subnet_attachments.device_id").
		Where("network_members.user_id = ? AND subnet_attachments.status = ? AND subnet_attachments.virtual_ip <> ''", userID, "active").
		Count(&count).Error
	return count, err
}

func (r *PostgresRepository) CountActiveAttachmentsByNetwork(ctx context.Context, networkID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("subnet_attachments").
		Where("network_id = ? AND status = ? AND virtual_ip <> ''", networkID, "active").
		Count(&count).Error
	return count, err
}
