package service

import "context"

func (s NetworkAccessService) ListPublicMappings(ctx context.Context, networkID string) ([]PublicMappingView, error) {
	items, err := s.Networks.ListPublicMappings(ctx, normalizeNetworkID(networkID))
	if err != nil {
		return nil, err
	}
	return publicMappingViews(items), nil
}

func (s NetworkAccessService) CreatePublicMapping(ctx context.Context, input CreatePublicMappingInput) (PublicMappingView, error) {
	input = normalizeCreatePublicMappingInput(input)
	if input.NetworkID == "" || input.Name == "" {
		return PublicMappingView{}, ErrInvalidArgument
	}
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID); err != nil {
		return PublicMappingView{}, err
	}
	now := networkNow(s.Now).Unix()
	item := newManagedPublicMapping(newManagedPublicMappingID(s.Networks, s.NewPublicMappingID), now, input)
	if err := s.Networks.SavePublicMapping(ctx, item); err != nil {
		return PublicMappingView{}, err
	}
	return publicMappingView(item), nil
}

func (s NetworkAccessService) UpdatePublicMapping(ctx context.Context, input UpdatePublicMappingInput) (PublicMappingView, error) {
	input = normalizeUpdatePublicMappingInput(input)
	if input.MappingID == "" {
		return PublicMappingView{}, ErrInvalidArgument
	}
	item, err := requireOwnedManagedPublicMapping(ctx, s.Users, s.Networks, input.ActorUserID, input.MappingID)
	if err != nil {
		return PublicMappingView{}, err
	}
	item = applyUpdatePublicMappingInput(item, input, networkNow(s.Now).Unix())
	if err := s.Networks.SavePublicMapping(ctx, item); err != nil {
		return PublicMappingView{}, err
	}
	return publicMappingView(item), nil
}

func (s NetworkAccessService) DeletePublicMapping(ctx context.Context, input DeletePublicMappingInput) error {
	input = normalizeDeletePublicMappingInput(input)
	if input.MappingID == "" {
		return ErrInvalidArgument
	}
	if _, err := requireOwnedManagedPublicMapping(ctx, s.Users, s.Networks, input.ActorUserID, input.MappingID); err != nil {
		return err
	}
	return s.Networks.DeletePublicMapping(ctx, input.MappingID)
}

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
	return securityGroupView(item), nil
}

func (s NetworkAccessService) DeleteSecurityGroup(ctx context.Context, input DeleteSecurityGroupInput) error {
	input = normalizeDeleteSecurityGroupInput(input)
	if input.SecurityGroupID == "" {
		return ErrInvalidArgument
	}
	if _, err := requireOwnedManagedSecurityGroup(ctx, s.Users, s.Networks, input.ActorUserID, input.SecurityGroupID); err != nil {
		return err
	}
	return s.Networks.DeleteSecurityGroup(ctx, input.SecurityGroupID)
}

func (s NetworkAccessService) ListSecurityRules(ctx context.Context, securityGroupID string) ([]SecurityRuleView, error) {
	items, err := s.Networks.ListSecurityRules(ctx, normalizeSecurityGroupID(securityGroupID))
	if err != nil {
		return nil, err
	}
	return securityRuleViews(items), nil
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
	now := networkNow(s.Now).Unix()
	item := newManagedSecurityRule(newManagedSecurityRuleID(s.Networks, s.NewSecurityRuleID), now, input)
	if err := s.Networks.SaveSecurityRule(ctx, item); err != nil {
		return SecurityRuleView{}, err
	}
	return securityRuleView(item), nil
}

func (s NetworkAccessService) UpdateSecurityRule(ctx context.Context, input UpdateSecurityRuleInput) (SecurityRuleView, error) {
	input = normalizeUpdateSecurityRuleInput(input)
	if input.RuleID == "" {
		return SecurityRuleView{}, ErrInvalidArgument
	}
	item, _, err := requireOwnedManagedSecurityRule(ctx, s.Users, s.Networks, input.ActorUserID, input.RuleID)
	if err != nil {
		return SecurityRuleView{}, err
	}
	item = applyUpdateSecurityRuleInput(item, input, networkNow(s.Now).Unix())
	if err := s.Networks.SaveSecurityRule(ctx, item); err != nil {
		return SecurityRuleView{}, err
	}
	return securityRuleView(item), nil
}

func (s NetworkAccessService) DeleteSecurityRule(ctx context.Context, input DeleteSecurityRuleInput) error {
	input = normalizeDeleteSecurityRuleInput(input)
	if input.RuleID == "" {
		return ErrInvalidArgument
	}
	if _, _, err := requireOwnedManagedSecurityRule(ctx, s.Users, s.Networks, input.ActorUserID, input.RuleID); err != nil {
		return err
	}
	return s.Networks.DeleteSecurityRule(ctx, input.RuleID)
}
