package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

type Merchant struct {
	MerchantID   string `gorm:"column:merchant_id;primaryKey"`
	MerchantCode string `gorm:"column:merchant_code;uniqueIndex;not null"`
	MerchantName string `gorm:"column:merchant_name;not null"`
	ContactEmail string `gorm:"column:contact_email;not null;default:''"`
	Status       string `gorm:"column:status;not null;default:'active'"`
	CreatedAt    int64  `gorm:"column:created_at;not null;default:0"`
	UpdatedAt    int64  `gorm:"column:updated_at;not null;default:0"`
}

func (Merchant) TableName() string { return "merchants" }

type Product struct {
	ProductID          string `gorm:"column:product_id;primaryKey"`
	MerchantID         string `gorm:"column:merchant_id;index;not null;default:'merchant-platform'"`
	MerchantName       string `gorm:"column:merchant_name;not null;default:'SLAN Platform'"`
	ProductCode        string `gorm:"column:product_code;uniqueIndex;not null"`
	ProductName        string `gorm:"column:product_name;not null"`
	Description        string `gorm:"column:description;not null;default:''"`
	ProductType        string `gorm:"column:product_type;index;not null;default:'plan'"`
	PriceCents         int64  `gorm:"column:price_cents;not null;default:0"`
	Currency           string `gorm:"column:currency;not null;default:'CNY'"`
	BillingCycle       string `gorm:"column:billing_cycle;not null;default:'month'"`
	UnitQuantity       int    `gorm:"column:unit_quantity;not null;default:1"`
	MaxActiveDevices   int    `gorm:"column:max_active_devices;not null;default:0"`
	BandwidthLimitMbps int    `gorm:"column:bandwidth_limit_mbps;not null;default:0"`
	IsDefault          bool   `gorm:"column:is_default;not null;default:false"`
	Status             string `gorm:"column:status;not null;default:'active'"`
	CreatedAt          int64  `gorm:"column:created_at;not null;default:0"`
	UpdatedAt          int64  `gorm:"column:updated_at;not null;default:0"`
}

func (Product) TableName() string { return "products" }

type PurchaseOrder struct {
	OrderID      string `gorm:"column:order_id;primaryKey"`
	UserID       string `gorm:"column:user_id;index;not null"`
	UserEmail    string `gorm:"column:user_email;index;not null;default:''"`
	MerchantID   string `gorm:"column:merchant_id;index;not null;default:'merchant-platform'"`
	MerchantName string `gorm:"column:merchant_name;not null;default:'SLAN Platform'"`
	ProductID    string `gorm:"column:product_id;index;not null;default:''"`
	ProductCode  string `gorm:"column:product_code;index;not null"`
	ProductName  string `gorm:"column:product_name;not null"`
	ProductType  string `gorm:"column:product_type;index;not null;default:''"`
	Quantity     int    `gorm:"column:quantity;not null;default:1"`
	Months       int    `gorm:"column:months;not null;default:1"`
	UnitCents    int64  `gorm:"column:unit_cents;not null;default:0"`
	AmountCents  int64  `gorm:"column:amount_cents;not null;default:0"`
	Currency     string `gorm:"column:currency;not null;default:'CNY'"`
	BillingCycle string `gorm:"column:billing_cycle;not null;default:'month'"`
	Status       string `gorm:"column:status;index;not null;default:'pending'"`
	PaidAt       int64  `gorm:"column:paid_at;not null;default:0"`
	CancelledAt  int64  `gorm:"column:cancelled_at;not null;default:0"`
	RefundedAt   int64  `gorm:"column:refunded_at;not null;default:0"`
	ExpiresAt    int64  `gorm:"column:expires_at;index;not null;default:0"`
	CreatedAt    int64  `gorm:"column:created_at;not null;default:0"`
	UpdatedAt    int64  `gorm:"column:updated_at;not null;default:0"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

type PurchaseOrderDeviceBinding struct {
	BindingID string `gorm:"column:binding_id;primaryKey"`
	OrderID   string `gorm:"column:order_id;index;not null;uniqueIndex:idx_order_device_binding"`
	UserID    string `gorm:"column:user_id;index;not null"`
	DeviceID  string `gorm:"column:device_id;index;not null;uniqueIndex:idx_order_device_binding"`
	BoundAt   int64  `gorm:"column:bound_at;not null;default:0"`
	ExpiresAt int64  `gorm:"column:expires_at;index;not null;default:0"`
	CreatedAt int64  `gorm:"column:created_at;not null;default:0"`
	UpdatedAt int64  `gorm:"column:updated_at;not null;default:0"`
}

func (PurchaseOrderDeviceBinding) TableName() string { return "purchase_order_device_bindings" }

func (r *PostgresRepository) UpsertMerchant(ctx context.Context, record Merchant) error {
	return r.db.WithContext(ctx).
		Where("merchant_code = ?", record.MerchantCode).
		Assign(map[string]any{
			"merchant_name": record.MerchantName,
			"contact_email": record.ContactEmail,
			"status":        record.Status,
			"updated_at":    record.UpdatedAt,
		}).
		FirstOrCreate(&record).Error
}

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

func (r *PostgresRepository) GetProductByID(ctx context.Context, productID string) (Product, error) {
	var record Product
	err := r.db.WithContext(ctx).Where("product_id = ?", productID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) GetProductByCode(ctx context.Context, productCode string) (Product, error) {
	var record Product
	err := r.db.WithContext(ctx).Where("product_code = ?", productCode).First(&record).Error
	return record, err
}

func (r *PostgresRepository) GetDefaultProduct(ctx context.Context) (Product, error) {
	var record Product
	err := r.db.WithContext(ctx).
		Where("is_default = ? AND status = ?", true, "active").
		Order("product_code").
		First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListProducts(ctx context.Context) ([]Product, error) {
	var out []Product
	err := r.db.WithContext(ctx).Order("is_default desc, product_code").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) UpsertProduct(ctx context.Context, record Product) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if record.IsDefault {
			if err := tx.Model(&Product{}).
				Where("product_code <> ?", record.ProductCode).
				Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "product_code"}},
			DoUpdates: clause.Assignments(map[string]any{
				"merchant_id":          record.MerchantID,
				"merchant_name":        record.MerchantName,
				"product_name":         record.ProductName,
				"description":          record.Description,
				"product_type":         record.ProductType,
				"price_cents":          record.PriceCents,
				"currency":             record.Currency,
				"billing_cycle":        record.BillingCycle,
				"unit_quantity":        record.UnitQuantity,
				"max_active_devices":   record.MaxActiveDevices,
				"bandwidth_limit_mbps": record.BandwidthLimitMbps,
				"is_default":           record.IsDefault,
				"status":               record.Status,
				"updated_at":           record.UpdatedAt,
			}),
		}).Create(&record).Error
	})
}

func (r *PostgresRepository) CreatePurchaseOrder(ctx context.Context, record PurchaseOrder) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) GetPurchaseOrderByID(ctx context.Context, orderID string) (PurchaseOrder, error) {
	var record PurchaseOrder
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListPurchaseOrders(ctx context.Context) ([]PurchaseOrder, error) {
	var out []PurchaseOrder
	err := r.db.WithContext(ctx).
		Order("created_at desc, order_id desc").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListPurchaseOrdersByUser(ctx context.Context, userID string) ([]PurchaseOrder, error) {
	var out []PurchaseOrder
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at desc, order_id desc").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListPurchaseOrdersByUserProduct(ctx context.Context, userID, productCode string) ([]PurchaseOrder, error) {
	var out []PurchaseOrder
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND product_code = ?", userID, productCode).
		Order("created_at desc, order_id desc").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) UpdatePurchaseOrderStatus(ctx context.Context, orderID, status string, updatedAt int64) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": updatedAt,
	}
	switch status {
	case "paid", "active", "completed":
		updates["paid_at"] = updatedAt
	case "cancelled":
		updates["cancelled_at"] = updatedAt
	case "refunded":
		updates["refunded_at"] = updatedAt
	}
	tx := r.db.WithContext(ctx).
		Model(&PurchaseOrder{}).
		Where("order_id = ?", orderID).
		Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PostgresRepository) UpdatePurchaseOrderExpiresAt(ctx context.Context, orderID string, expiresAt int64) error {
	return r.db.WithContext(ctx).
		Model(&PurchaseOrder{}).
		Where("order_id = ?", orderID).
		Update("expires_at", expiresAt).Error
}

func (r *PostgresRepository) CreatePurchaseOrderDeviceBinding(ctx context.Context, record PurchaseOrderDeviceBinding) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) ListPurchaseOrderDeviceBindingsByUser(ctx context.Context, userID string) ([]PurchaseOrderDeviceBinding, error) {
	var out []PurchaseOrderDeviceBinding
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at asc, binding_id asc").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) MaxPurchaseOrderDeviceBindingExpiresAt(ctx context.Context, orderID string) (int64, error) {
	var expiresAt int64
	err := r.db.WithContext(ctx).
		Model(&PurchaseOrderDeviceBinding{}).
		Where("order_id = ?", orderID).
		Select("COALESCE(MAX(expires_at), 0)").
		Scan(&expiresAt).Error
	return expiresAt, err
}

func (r *PostgresRepository) CountActiveAttachmentsByUser(ctx context.Context, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("subnet_attachments").
		Joins("join devices on devices.device_id = subnet_attachments.device_id").
		Where("devices.user_id = ? AND subnet_attachments.status = ? AND subnet_attachments.virtual_ip <> ''", userID, "active").
		Count(&count).Error
	return count, err
}
