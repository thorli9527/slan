package service

import "github.com/slan/service-biz/internal/model"

func newManagedSecurityGroup(id string, now int64, input CreateSecurityGroupInput) model.SecurityGroup {
	return model.SecurityGroup{
		SecurityGroupID: id,
		NetworkID:       input.NetworkID,
		Name:            input.Name,
		Description:     input.Description,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func applyUpdateSecurityGroupInput(item model.SecurityGroup, input UpdateSecurityGroupInput, now int64) model.SecurityGroup {
	if input.Name != "" {
		item.Name = input.Name
	}
	item.Description = input.Description
	item.UpdatedAt = now
	return item
}

func newManagedSecurityRule(id string, now int64, input CreateSecurityRuleInput) model.SecurityRule {
	return model.SecurityRule{
		RuleID:          id,
		SecurityGroupID: input.SecurityGroupID,
		Direction:       input.Direction,
		Protocol:        input.Protocol,
		PortRange:       input.PortRange,
		CIDR:            securityRuleLegacyCIDR(input.PeerType, input.PeerValue),
		PeerType:        input.PeerType,
		PeerValue:       input.PeerValue,
		Action:          input.Action,
		Priority:        input.Priority,
		Description:     input.Description,
		Enabled:         input.Enabled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func applyUpdateSecurityRuleInput(item model.SecurityRule, input UpdateSecurityRuleInput, now int64) model.SecurityRule {
	if input.Direction != "" {
		item.Direction = input.Direction
	}
	if input.Protocol != "" {
		item.Protocol = input.Protocol
	}
	if input.PortRange != "" {
		item.PortRange = input.PortRange
	}
	if input.PeerType != "" {
		item.PeerType = input.PeerType
		item.PeerValue = input.PeerValue
		item.CIDR = securityRuleLegacyCIDR(input.PeerType, input.PeerValue)
	}
	if input.Action != "" {
		item.Action = input.Action
	}
	if input.Priority != nil {
		item.Priority = *input.Priority
	}
	item.Description = input.Description
	if input.Enabled != nil {
		item.Enabled = *input.Enabled
	}
	item.UpdatedAt = now
	return item
}
