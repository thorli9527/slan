package biz

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) CreateSecurityGroup(networkID, name, description, defaultPolicy string) (SecurityGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return SecurityGroup{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.networks[networkID]; !ok {
		return SecurityGroup{}, errNotFound
	}
	group := s.addSecurityGroupLocked(networkID, name, description, defaultString(defaultPolicy, "deny"), time.Now().Unix())
	if err := s.persistPostgresSecurityGroupUpsertTxLocked(ctx, postgresTx, group); err != nil {
		return SecurityGroup{}, err
	}
	postgresTx = nil
	return group, nil
}

func (s *Store) DeleteSecurityGroup(networkID, securityGroupID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	group, ok := s.securityGroups[securityGroupID]
	if !ok || group.NetworkID != networkID {
		return errNotFound
	}
	delete(s.securityGroups, securityGroupID)
	for ruleID, rule := range s.securityGroupRules {
		if rule.SecurityGroupID == securityGroupID {
			delete(s.securityGroupRules, ruleID)
		}
	}
	if err := s.persistPostgresSecurityGroupDeleteTxLocked(ctx, postgresTx, networkID, securityGroupID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) AddSecurityGroupRule(securityGroupID, direction, action, protocol, peerType, peerValue, description string, priority, portFrom, portTo int, enabled bool) (SecurityGroupRule, error) {
	if securityGroupID == "" || direction == "" || action == "" {
		return SecurityGroupRule{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return SecurityGroupRule{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.securityGroups[securityGroupID]; !ok {
		return SecurityGroupRule{}, errNotFound
	}
	rule := SecurityGroupRule{
		RuleID:          fmt.Sprintf("sgr-%06d", s.nextSecurityRuleSeq),
		SecurityGroupID: securityGroupID,
		Direction:       direction,
		Priority:        defaultInt(priority, 100),
		Action:          action,
		Protocol:        defaultString(protocol, "all"),
		PortFrom:        portFrom,
		PortTo:          portTo,
		PeerType:        defaultString(peerType, "network"),
		PeerValue:       strings.TrimSpace(peerValue),
		Description:     strings.TrimSpace(description),
		Enabled:         enabled,
		CreatedAt:       time.Now().Unix(),
	}
	s.nextSecurityRuleSeq++
	s.securityGroupRules[rule.RuleID] = rule
	if err := s.persistPostgresSecurityGroupRuleUpsertTxLocked(ctx, postgresTx, rule); err != nil {
		return SecurityGroupRule{}, err
	}
	postgresTx = nil
	return rule, nil
}

func (s *Store) UpdateSecurityGroupRule(ruleID, direction, action, protocol, peerType, peerValue, description string, priority, portFrom, portTo int, enabled bool) (SecurityGroupRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return SecurityGroupRule{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	rule, ok := s.securityGroupRules[ruleID]
	if !ok {
		return SecurityGroupRule{}, errNotFound
	}
	rule.Direction = defaultString(direction, rule.Direction)
	rule.Priority = defaultInt(priority, rule.Priority)
	rule.Action = defaultString(action, rule.Action)
	rule.Protocol = defaultString(protocol, rule.Protocol)
	rule.PortFrom = portFrom
	rule.PortTo = portTo
	rule.PeerType = defaultString(peerType, rule.PeerType)
	rule.PeerValue = strings.TrimSpace(peerValue)
	rule.Description = strings.TrimSpace(description)
	rule.Enabled = enabled
	s.securityGroupRules[ruleID] = rule
	if err := s.persistPostgresSecurityGroupRuleUpsertTxLocked(ctx, postgresTx, rule); err != nil {
		return SecurityGroupRule{}, err
	}
	postgresTx = nil
	return rule, nil
}

func (s *Store) DeleteSecurityGroupRule(ruleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.securityGroupRules[ruleID]; !ok {
		return errNotFound
	}
	delete(s.securityGroupRules, ruleID)
	if err := s.persistPostgresSecurityGroupRuleDeleteTxLocked(ctx, postgresTx, ruleID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) SecurityGroupNetworkID(securityGroupID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	group, ok := s.securityGroups[securityGroupID]
	if !ok {
		return "", errNotFound
	}
	return group.NetworkID, nil
}

func (s *Store) SecurityRuleNetworkID(ruleID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, ok := s.securityGroupRules[ruleID]
	if !ok {
		return "", errNotFound
	}
	group, ok := s.securityGroups[rule.SecurityGroupID]
	if !ok {
		return "", errNotFound
	}
	return group.NetworkID, nil
}

func (s *Store) GetSecurityRule(ruleID string) (SecurityGroupRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return SecurityGroupRule{}, err
	}
	rule, ok := s.securityGroupRules[ruleID]
	if !ok {
		return SecurityGroupRule{}, errNotFound
	}
	return rule, nil
}

func (s *Store) ListSecurityGroups(networkID string) []SecurityGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres security groups failed: %v", err)
	}
	out := make([]SecurityGroup, 0)
	for _, group := range s.securityGroups {
		if networkID == "" || group.NetworkID == networkID {
			out = append(out, group)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SecurityGroupID < out[j].SecurityGroupID })
	return out
}

func (s *Store) ListSecurityGroupRules(securityGroupID string) []SecurityGroupRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres security rules failed: %v", err)
	}
	out := make([]SecurityGroupRule, 0)
	for _, rule := range s.securityGroupRules {
		if securityGroupID == "" || rule.SecurityGroupID == securityGroupID {
			out = append(out, rule)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Priority < out[j].Priority })
	return out
}
