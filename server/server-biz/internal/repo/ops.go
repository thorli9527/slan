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
	// DisplayName 是管理员显示名称。
	DisplayName string `gorm:"column:display_name;not null"`
	// Phone 是联系电话。
	Phone string `gorm:"column:phone;not null;default:''"`
	// Status 是管理员状态。
	Status string `gorm:"column:status;not null;default:'active'"`
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

func (r *PostgresRepository) UpsertAdminInfo(ctx context.Context, record AdminInfo) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", record.UserID).
		Assign(map[string]any{
			"display_name": record.DisplayName,
			"phone":        record.Phone,
			"status":       record.Status,
		}).
		FirstOrCreate(&record).Error
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
