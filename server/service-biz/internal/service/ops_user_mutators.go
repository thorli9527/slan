package service

import "github.com/slan/service-biz/internal/model"

func applyUpdateUserInput(user model.User, input UpdateUserInput, now int64) model.User {
	if input.Email != "" {
		user.Email = input.Email
	}
	if input.Name != "" {
		user.Name = input.Name
	}
	user.Country = input.Country
	user.Province = input.Province
	user.City = input.City
	user.IPRegion = input.IPRegion
	if input.Status != "" {
		user.Status = input.Status
	}
	user.UpdatedAt = now
	return user
}
