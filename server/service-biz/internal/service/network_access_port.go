package service

import "context"

type NetworkAccessUseCase interface {
	ListSecurityGroups(ctx context.Context, networkID string) ([]SecurityGroupView, error)
	CreateSecurityGroup(ctx context.Context, input CreateSecurityGroupInput) (SecurityGroupView, error)
	UpdateSecurityGroup(ctx context.Context, input UpdateSecurityGroupInput) (SecurityGroupView, error)
	DeleteSecurityGroup(ctx context.Context, input DeleteSecurityGroupInput) error
	ListSecurityRules(ctx context.Context, securityGroupID string) ([]SecurityRuleView, error)
	AddSecurityRule(ctx context.Context, input CreateSecurityRuleInput) (SecurityRuleView, error)
	UpdateSecurityRule(ctx context.Context, input UpdateSecurityRuleInput) (SecurityRuleView, error)
	DeleteSecurityRule(ctx context.Context, input DeleteSecurityRuleInput) error
}
