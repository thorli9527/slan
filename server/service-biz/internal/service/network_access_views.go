package service

import (
	"github.com/slan/service-biz/internal/model"
)

func securityGroupView(item model.SecurityGroup) SecurityGroupView {
	return SecurityGroupView{
		SecurityGroupID: item.SecurityGroupID,
		NetworkID:       item.NetworkID,
		Name:            item.Name,
		Description:     item.Description,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func securityRuleView(item model.SecurityRule) SecurityRuleView {
	portFrom, portTo := parsePortRange(item.PortRange)
	peerType, peerValue := normalizedSecurityPeer(item.PeerType, item.PeerValue, item.CIDR)
	return SecurityRuleView{
		RuleID:          item.RuleID,
		SecurityGroupID: item.SecurityGroupID,
		Direction:       item.Direction,
		Protocol:        item.Protocol,
		PortRange:       item.PortRange,
		CIDR:            item.CIDR,
		Action:          item.Action,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		Priority:        item.Priority,
		PortFrom:        portFrom,
		PortTo:          portTo,
		PeerType:        peerType,
		PeerValue:       peerValue,
		Description:     item.Description,
		Enabled:         item.Enabled,
	}
}
