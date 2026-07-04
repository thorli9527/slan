package service

import "github.com/slan/service-biz/internal/model"

func newManagedPublicMapping(id string, now int64, input CreatePublicMappingInput) model.PublicMapping {
	return model.PublicMapping{
		MappingID:    id,
		NetworkID:    input.NetworkID,
		Name:         input.Name,
		PublicDomain: input.PublicDomain,
		SourceRecord: input.SourceRecord,
		DeviceID:     input.DeviceID,
		Protocol:     input.Protocol,
		InternalIP:   input.InternalIP,
		InternalPort: input.InternalPort,
		ExternalPort: input.ExternalPort,
		AccessMode:   input.AccessMode,
		TLSMode:      input.TLSMode,
		Status:       "enabled",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func applyUpdatePublicMappingInput(item model.PublicMapping, input UpdatePublicMappingInput, now int64) model.PublicMapping {
	if input.Name != "" {
		item.Name = input.Name
	}
	item.PublicDomain = input.PublicDomain
	item.DeviceID = input.DeviceID
	item.SourceRecord = input.SourceRecord
	if input.Protocol != "" {
		item.Protocol = input.Protocol
	}
	item.InternalIP = input.InternalIP
	if input.InternalPort > 0 {
		item.InternalPort = input.InternalPort
	}
	if input.ExternalPort > 0 {
		item.ExternalPort = input.ExternalPort
	}
	item.AccessMode = input.AccessMode
	item.TLSMode = input.TLSMode
	if input.Status != "" {
		item.Status = normalizePublicMappingStatus(input.Status)
	}
	item.UpdatedAt = now
	return item
}

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
