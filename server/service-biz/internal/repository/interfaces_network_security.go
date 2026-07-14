package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type NetworkSecurityRepository interface {
	ListSecurityGroups(ctx context.Context, networkID string) ([]model.SecurityGroup, error)
	GetSecurityGroup(ctx context.Context, securityGroupID string) (model.SecurityGroup, bool, error)
	SaveSecurityGroup(ctx context.Context, group model.SecurityGroup) error
	DeleteSecurityGroup(ctx context.Context, securityGroupID string) error
	ListSecurityRules(ctx context.Context, securityGroupID string) ([]model.SecurityRule, error)
	GetSecurityRule(ctx context.Context, ruleID string) (model.SecurityRule, bool, error)
	SaveSecurityRule(ctx context.Context, rule model.SecurityRule) error
	DeleteSecurityRule(ctx context.Context, ruleID string) error
}
