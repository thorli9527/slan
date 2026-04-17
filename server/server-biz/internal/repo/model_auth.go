package repo

import "context"

type User struct {
	UserID       string `gorm:"column:user_id;primaryKey"`
	Email        string `gorm:"column:email;uniqueIndex;not null"`
	PasswordHash string `gorm:"column:password_hash;not null"`
}

func (User) TableName() string { return "users" }

func (r *PostgresRepository) CreateUser(ctx context.Context, user User) error {
	user.Email = normalizeEmail(user.Email)
	return r.db.WithContext(ctx).Create(&user).Error
}

func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("email = ?", normalizeEmail(email)).First(&user).Error
	return user, err
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, userID string) (User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&user).Error
	return user, err
}
