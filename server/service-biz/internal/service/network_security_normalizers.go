package service

import "strings"

func normalizeCreateSecurityGroupInput(input CreateSecurityGroupInput) CreateSecurityGroupInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeUpdateSecurityGroupInput(input UpdateSecurityGroupInput) UpdateSecurityGroupInput {
	input.SecurityGroupID = strings.TrimSpace(input.SecurityGroupID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeSecurityGroupID(securityGroupID string) string {
	return strings.TrimSpace(securityGroupID)
}

func normalizeCreateSecurityRuleInput(input CreateSecurityRuleInput) CreateSecurityRuleInput {
	input.SecurityGroupID = strings.TrimSpace(input.SecurityGroupID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Direction = strings.TrimSpace(input.Direction)
	input.Protocol = strings.TrimSpace(input.Protocol)
	input.PortRange = strings.TrimSpace(input.PortRange)
	input.CIDR = strings.TrimSpace(input.CIDR)
	input.Action = strings.TrimSpace(input.Action)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeUpdateSecurityRuleInput(input UpdateSecurityRuleInput) UpdateSecurityRuleInput {
	input.RuleID = strings.TrimSpace(input.RuleID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Direction = strings.TrimSpace(input.Direction)
	input.Protocol = strings.TrimSpace(input.Protocol)
	input.PortRange = strings.TrimSpace(input.PortRange)
	input.CIDR = strings.TrimSpace(input.CIDR)
	input.Action = strings.TrimSpace(input.Action)
	input.Description = strings.TrimSpace(input.Description)
	return input
}

func normalizeSecurityRuleID(ruleID string) string {
	return strings.TrimSpace(ruleID)
}

func normalizeDeleteSecurityGroupInput(input DeleteSecurityGroupInput) DeleteSecurityGroupInput {
	input.SecurityGroupID = strings.TrimSpace(input.SecurityGroupID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}

func normalizeDeleteSecurityRuleInput(input DeleteSecurityRuleInput) DeleteSecurityRuleInput {
	input.RuleID = strings.TrimSpace(input.RuleID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}
