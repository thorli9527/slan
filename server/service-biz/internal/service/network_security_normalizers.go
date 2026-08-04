package service

import "strings"

func normalizeCreateSecurityGroupInput(input CreateSecurityGroupInput) CreateSecurityGroupInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeUpdateSecurityGroupInput(input UpdateSecurityGroupInput) UpdateSecurityGroupInput {
	input.SecurityGroupID = strings.TrimSpace(input.SecurityGroupID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeSecurityGroupID(securityGroupID string) string {
	return strings.TrimSpace(securityGroupID)
}

func normalizeCreateSecurityRuleInput(input CreateSecurityRuleInput) CreateSecurityRuleInput {
	input.SecurityGroupID = strings.TrimSpace(input.SecurityGroupID)
	input.Direction = strings.TrimSpace(input.Direction)
	input.Protocol = strings.TrimSpace(input.Protocol)
	input.PortRange = strings.TrimSpace(input.PortRange)
	input.PeerType = strings.TrimSpace(input.PeerType)
	input.PeerValue = strings.TrimSpace(input.PeerValue)
	input.Action = strings.TrimSpace(input.Action)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeUpdateSecurityRuleInput(input UpdateSecurityRuleInput) UpdateSecurityRuleInput {
	input.RuleID = strings.TrimSpace(input.RuleID)
	input.Direction = strings.TrimSpace(input.Direction)
	input.Protocol = strings.TrimSpace(input.Protocol)
	input.PortRange = strings.TrimSpace(input.PortRange)
	input.PeerType = strings.TrimSpace(input.PeerType)
	input.PeerValue = strings.TrimSpace(input.PeerValue)
	input.Action = strings.TrimSpace(input.Action)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeSecurityRuleID(ruleID string) string {
	return strings.TrimSpace(ruleID)
}

func normalizeDeleteSecurityGroupInput(input DeleteSecurityGroupInput) DeleteSecurityGroupInput {
	input.SecurityGroupID = strings.TrimSpace(input.SecurityGroupID)
	return input
}

func normalizeDeleteSecurityRuleInput(input DeleteSecurityRuleInput) DeleteSecurityRuleInput {
	input.RuleID = strings.TrimSpace(input.RuleID)
	return input
}
