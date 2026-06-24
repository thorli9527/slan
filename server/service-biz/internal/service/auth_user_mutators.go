package service

import (
	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
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

func newDefaultUserNetwork(newNetID func() string, ownerID string, now int64) model.Network {
	return model.Network{
		NetworkID:        newAuthNetworkID(newNetID),
		OwnerID:          ownerID,
		Name:             "Default Network",
		Code:             "default",
		TemplateKey:      "default",
		IntraGroupPolicy: "allow",
		Default:          true,
		CIDR:             "10.0.0.0/24",
		Status:           "active",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func newDefaultUserSecurityGroup(newSecurityGroupID func() string, networks repository.NetworkRepository, networkID string, now int64) model.SecurityGroup {
	return model.SecurityGroup{
		SecurityGroupID: newManagedSecurityGroupID(networks, newSecurityGroupID),
		NetworkID:       networkID,
		Name:            "Default Security Group",
		Description:     "Default security group for the network",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func applyChangedUserPassword(user model.User, newPassword string, now int64) model.User {
	user.PasswordHash = hashPassword(newPassword)
	user.UpdatedAt = now
	return user
}
