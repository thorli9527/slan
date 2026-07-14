package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s NetworkAccessService) ListSecurityGroups(ctx context.Context, networkID string) ([]SecurityGroupView, error) {
	items, err := s.Networks.ListSecurityGroups(ctx, normalizeNetworkID(networkID))
	if err != nil {
		return nil, err
	}
	return securityGroupViews(items), nil
}

func (s NetworkAccessService) CreateSecurityGroup(ctx context.Context, input CreateSecurityGroupInput) (SecurityGroupView, error) {
	input = normalizeCreateSecurityGroupInput(input)
	if input.NetworkID == "" {
		return SecurityGroupView{}, ErrInvalidArgument
	}
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID); err != nil {
		return SecurityGroupView{}, err
	}
	now := networkNow(s.Now).Unix()
	item := newManagedSecurityGroup(newManagedSecurityGroupID(s.Networks, s.NewSecurityGroupID), now, input)
	if err := s.Networks.SaveSecurityGroup(ctx, item); err != nil {
		return SecurityGroupView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, "security_group_created")
	if err != nil {
		return SecurityGroupView{}, err
	}
	if err := publishACLChanged(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return SecurityGroupView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return SecurityGroupView{}, err
	}
	return securityGroupView(item), nil
}

func (s NetworkAccessService) UpdateSecurityGroup(ctx context.Context, input UpdateSecurityGroupInput) (SecurityGroupView, error) {
	input = normalizeUpdateSecurityGroupInput(input)
	if input.SecurityGroupID == "" {
		return SecurityGroupView{}, ErrInvalidArgument
	}
	item, err := requireOwnedManagedSecurityGroup(ctx, s.Users, s.Networks, input.ActorUserID, input.SecurityGroupID)
	if err != nil {
		return SecurityGroupView{}, err
	}
	item = applyUpdateSecurityGroupInput(item, input, networkNow(s.Now).Unix())
	if err := s.Networks.SaveSecurityGroup(ctx, item); err != nil {
		return SecurityGroupView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, "security_group_updated")
	if err != nil {
		return SecurityGroupView{}, err
	}
	if err := publishACLChanged(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return SecurityGroupView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return SecurityGroupView{}, err
	}
	return securityGroupView(item), nil
}

func (s NetworkAccessService) DeleteSecurityGroup(ctx context.Context, input DeleteSecurityGroupInput) error {
	input = normalizeDeleteSecurityGroupInput(input)
	if input.SecurityGroupID == "" {
		return ErrInvalidArgument
	}
	group, err := requireOwnedManagedSecurityGroup(ctx, s.Users, s.Networks, input.ActorUserID, input.SecurityGroupID)
	if err != nil {
		return err
	}
	if err := s.Networks.DeleteSecurityGroup(ctx, input.SecurityGroupID); err != nil {
		return err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, group.NetworkID, "security_group_deleted")
	if err != nil {
		return err
	}
	if err := publishACLChanged(ctx, s.Networks, s.EventPublisher, s.Now, group.NetworkID, version.Version, version.Reason); err != nil {
		return err
	}
	return publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, group.NetworkID, version.Version, version.Reason)
}

func (s NetworkAccessService) ListSecurityRules(ctx context.Context, securityGroupID string) ([]SecurityRuleView, error) {
	items, err := s.Networks.ListSecurityRules(ctx, normalizeSecurityGroupID(securityGroupID))
	if err != nil {
		return nil, err
	}
	groupRules := make([]model.SecurityRule, 0, len(items))
	for _, item := range items {
		if supportedSecurityRulePeerType(item.PeerType) {
			groupRules = append(groupRules, item)
		}
	}
	return securityRuleViews(groupRules), nil
}

func (s NetworkAccessService) AddSecurityRule(ctx context.Context, input CreateSecurityRuleInput) (SecurityRuleView, error) {
	input = normalizeCreateSecurityRuleInput(input)
	if input.SecurityGroupID == "" {
		return SecurityRuleView{}, ErrInvalidArgument
	}
	group, err := requireManagedSecurityGroup(ctx, s.Networks, input.SecurityGroupID)
	if err != nil {
		return SecurityRuleView{}, err
	}
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, group.NetworkID); err != nil {
		return SecurityRuleView{}, err
	}
	if err := validateSecurityRulePeer(ctx, s.Devices, s.Networks, input.ActorUserID, group.NetworkID, input.PeerType, input.PeerValue); err != nil {
		return SecurityRuleView{}, err
	}
	now := networkNow(s.Now).Unix()
	item := newManagedSecurityRule(newManagedSecurityRuleID(s.Networks, s.NewSecurityRuleID), now, input)
	if err := s.Networks.SaveSecurityRule(ctx, item); err != nil {
		return SecurityRuleView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, group.NetworkID, "security_rule_created")
	if err != nil {
		return SecurityRuleView{}, err
	}
	if err := publishACLChanged(ctx, s.Networks, s.EventPublisher, s.Now, group.NetworkID, version.Version, version.Reason); err != nil {
		return SecurityRuleView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, group.NetworkID, version.Version, version.Reason); err != nil {
		return SecurityRuleView{}, err
	}
	return securityRuleView(item), nil
}

func (s NetworkAccessService) UpdateSecurityRule(ctx context.Context, input UpdateSecurityRuleInput) (SecurityRuleView, error) {
	input = normalizeUpdateSecurityRuleInput(input)
	if input.RuleID == "" {
		return SecurityRuleView{}, ErrInvalidArgument
	}
	item, group, err := requireOwnedManagedSecurityRule(ctx, s.Users, s.Networks, input.ActorUserID, input.RuleID)
	if err != nil {
		return SecurityRuleView{}, err
	}
	if err := validateSecurityRulePeer(ctx, s.Devices, s.Networks, input.ActorUserID, group.NetworkID, input.PeerType, input.PeerValue); err != nil {
		return SecurityRuleView{}, err
	}
	item = applyUpdateSecurityRuleInput(item, input, networkNow(s.Now).Unix())
	if err := s.Networks.SaveSecurityRule(ctx, item); err != nil {
		return SecurityRuleView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, group.NetworkID, "security_rule_updated")
	if err != nil {
		return SecurityRuleView{}, err
	}
	if err := publishACLChanged(ctx, s.Networks, s.EventPublisher, s.Now, group.NetworkID, version.Version, version.Reason); err != nil {
		return SecurityRuleView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, group.NetworkID, version.Version, version.Reason); err != nil {
		return SecurityRuleView{}, err
	}
	return securityRuleView(item), nil
}

func (s NetworkAccessService) DeleteSecurityRule(ctx context.Context, input DeleteSecurityRuleInput) error {
	input = normalizeDeleteSecurityRuleInput(input)
	if input.RuleID == "" {
		return ErrInvalidArgument
	}
	_, group, err := requireOwnedManagedSecurityRule(ctx, s.Users, s.Networks, input.ActorUserID, input.RuleID)
	if err != nil {
		return err
	}
	if err := s.Networks.DeleteSecurityRule(ctx, input.RuleID); err != nil {
		return err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, group.NetworkID, "security_rule_deleted")
	if err != nil {
		return err
	}
	if err := publishACLChanged(ctx, s.Networks, s.EventPublisher, s.Now, group.NetworkID, version.Version, version.Reason); err != nil {
		return err
	}
	return publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, group.NetworkID, version.Version, version.Reason)
}
