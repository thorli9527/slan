package repo

import (
	"context"

	"github.com/slan/server/server-biz/internal/util"
)

// User 是终端业务用户的持久化模型。
//
// 这里存储的是控制面认证所需的最小字段：
// - 用户 ID
// - 规范化邮箱
// - 密码哈希
type User struct {
	UserID          string `gorm:"column:user_id;primaryKey"`
	Email           string `gorm:"column:email;uniqueIndex;not null"`
	PasswordHash    string `gorm:"column:password_hash;not null"`
	ActiveNetworkID string `gorm:"column:active_network_id;index"`
}

func (User) TableName() string { return "users" }

// CreateUser 创建一个新的用户记录，并在写入前统一规范化邮箱。
func (r *PostgresRepository) CreateUser(ctx context.Context, user User) error {
	user.Email = util.NormalizeEmail(user.Email)
	return r.db.WithContext(ctx).Create(&user).Error
}

// GetUserByEmail 使用规范化后的邮箱读取用户。
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("email = ?", util.NormalizeEmail(email)).First(&user).Error
	return user, err
}

// GetUserByID 按用户 ID 读取用户。
func (r *PostgresRepository) GetUserByID(ctx context.Context, userID string) (User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&user).Error
	return user, err
}

// UpdateUserActiveNetwork switches the user's current active network pointer.
func (r *PostgresRepository) UpdateUserActiveNetwork(ctx context.Context, userID, networkID string) error {
	return r.db.WithContext(ctx).
		Model(&User{}).
		Where("user_id = ?", userID).
		Update("active_network_id", networkID).Error
}

// ListUsers 返回当前所有用户，主要供 ops 视图聚合使用。
func (r *PostgresRepository) ListUsers(ctx context.Context) ([]User, error) {
	var out []User
	err := r.db.WithContext(ctx).Order("user_id").Find(&out).Error
	return out, err
}
