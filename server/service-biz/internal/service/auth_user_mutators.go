package service

import (
	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

const defaultUserDeviceGroupName = "开发部"

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

func newDefaultUserDeviceGroup(devices repository.DeviceRepository, userID string, now int64) model.DeviceGroup {
	return model.DeviceGroup{
		GroupID:     newDeviceGroupID(devices),
		UserID:      userID,
		Name:        defaultUserDeviceGroupName,
		Description: "默认设备组",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func newDefaultUserNetworkDeviceGroupReference(networkID, groupID string, now int64) model.NetworkDeviceGroupReference {
	return model.NetworkDeviceGroupReference{
		NetworkID: networkID,
		GroupID:   groupID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func newDefaultUserSecurityRules(networks repository.NetworkRepository, securityGroupID, deviceGroupID string, now int64) []model.SecurityRule {
	rules := make([]model.SecurityRule, 0, 2)
	for _, direction := range []string{"ingress", "egress"} {
		rules = append(rules, model.SecurityRule{
			RuleID:          newManagedSecurityRuleID(networks, nil),
			SecurityGroupID: securityGroupID,
			Direction:       direction,
			Protocol:        "all",
			PortRange:       "all",
			CIDR:            securityRuleLegacyCIDR("device_group", deviceGroupID),
			PeerType:        "device_group",
			PeerValue:       deviceGroupID,
			Action:          "allow",
			Priority:        100,
			Description:     "开发部全部协议互通",
			Enabled:         true,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}
	return rules
}

func applyChangedUserPassword(user model.User, newPassword string, now int64) model.User {
	user.PasswordHash = hashPassword(newPassword)
	user.UpdatedAt = now
	return user
}
