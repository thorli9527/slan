package service

import "context"

type NetworkAccessUseCase interface {
	ListPublicMappings(ctx context.Context, networkID string) ([]PublicMappingView, error)
	CreatePublicMapping(ctx context.Context, input CreatePublicMappingInput) (PublicMappingView, error)
	UpdatePublicMapping(ctx context.Context, input UpdatePublicMappingInput) (PublicMappingView, error)
	DeletePublicMapping(ctx context.Context, input DeletePublicMappingInput) error
	ListSecurityGroups(ctx context.Context, networkID string) ([]SecurityGroupView, error)
	CreateSecurityGroup(ctx context.Context, input CreateSecurityGroupInput) (SecurityGroupView, error)
	UpdateSecurityGroup(ctx context.Context, input UpdateSecurityGroupInput) (SecurityGroupView, error)
	DeleteSecurityGroup(ctx context.Context, input DeleteSecurityGroupInput) error
	ListSecurityRules(ctx context.Context, securityGroupID string) ([]SecurityRuleView, error)
	AddSecurityRule(ctx context.Context, input CreateSecurityRuleInput) (SecurityRuleView, error)
	UpdateSecurityRule(ctx context.Context, input UpdateSecurityRuleInput) (SecurityRuleView, error)
	DeleteSecurityRule(ctx context.Context, input DeleteSecurityRuleInput) error
}
