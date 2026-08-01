package service

import (
	"github.com/slan/service-biz/internal/model"
)

func newRegisteredUser(newUserID func() string, input RegisterUserInput, now int64) model.User {
	return model.User{
		UserID:       newAuthUserID(newUserID),
		Email:        input.Email,
		Name:         input.Name,
		PasswordHash: hashPassword(input.Password),
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func applyChangedUserPassword(user model.User, newPassword string, now int64) model.User {
	user.PasswordHash = hashPassword(newPassword)
	user.UpdatedAt = now
	return user
}
