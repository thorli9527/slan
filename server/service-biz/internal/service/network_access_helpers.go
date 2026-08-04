package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func parsePortRange(value string) (int, int) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "all") {
		return 0, 0
	}
	if strings.Contains(value, ",") {
		left, right, _ := strings.Cut(value, ",")
		portFrom, _ := strconv.Atoi(strings.TrimSpace(left))
		portTo, _ := strconv.Atoi(strings.TrimSpace(right))
		if portTo == 0 {
			portTo = portFrom
		}
		return portFrom, portTo
	}
	if !strings.Contains(value, "-") {
		port, _ := strconv.Atoi(value)
		return port, port
	}
	left, right, _ := strings.Cut(value, "-")
	portFrom, _ := strconv.Atoi(strings.TrimSpace(left))
	portTo, _ := strconv.Atoi(strings.TrimSpace(right))
	return portFrom, portTo
}

func securityPeer(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "all", "all"
	}
	if strings.HasPrefix(value, "peer:") {
		rest := strings.TrimPrefix(value, "peer:")
		peerType, peerValue, ok := strings.Cut(rest, ":")
		if !ok {
			return "cidr", value
		}
		peerType = strings.TrimSpace(peerType)
		peerValue = strings.TrimSpace(peerValue)
		if peerType == "" {
			return "cidr", value
		}
		if peerValue == "" && peerType == "all" {
			peerValue = "all"
		}
		return peerType, peerValue
	}
	return "cidr", value
}

func normalizedSecurityPeer(peerType, peerValue, fallbackCIDR string) (string, string) {
	peerType = strings.ToLower(strings.TrimSpace(peerType))
	peerValue = strings.TrimSpace(peerValue)
	if peerType != "" {
		if peerType == "any" {
			peerType = "all"
		}
		if peerType == "network" {
			peerType = "workspace"
		}
		if peerType == "device-group" {
			peerType = "device_group"
		}
		if peerType == "all" && peerValue == "" {
			peerValue = "all"
		}
		return peerType, peerValue
	}
	return securityPeer(fallbackCIDR)
}

func securityRuleLegacyCIDR(peerType, peerValue string) string {
	peerType = strings.ToLower(strings.TrimSpace(peerType))
	peerValue = strings.TrimSpace(peerValue)
	switch peerType {
	case "", "cidr":
		return peerValue
	case "all", "any":
		return "peer:all:all"
	default:
		return "peer:" + peerType + ":" + peerValue
	}
}

func validateSecurityRulePeer(
	ctx context.Context,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	networkID string,
	peerType string,
	peerValue string,
) error {
	peerType, peerValue = normalizedSecurityPeer(peerType, peerValue, "")
	networkID = normalizeNetworkID(networkID)
	if !supportedSecurityRulePeerType(peerType) || peerValue == "" || devices == nil || networks == nil || networkID == "" {
		return ErrInvalidArgument
	}
	if peerType == "device" {
		members, err := networks.ListNetworkDevices(ctx, networkID)
		if err != nil {
			return err
		}
		for _, member := range members {
			if member.DeviceID == peerValue && networkMemberActive(member) {
				return nil
			}
		}
		return ErrInvalidArgument
	}
	_, ok, err := devices.GetDeviceGroup(ctx, peerValue)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidArgument
	}
	groupReferences, ok := networks.(repository.NetworkDeviceGroupRepository)
	if !ok {
		return ErrNotImplemented
	}
	references, err := groupReferences.ListNetworkDeviceGroupReferences(ctx, networkID)
	if err != nil {
		return err
	}
	for _, reference := range references {
		if reference.GroupID == peerValue {
			return nil
		}
	}
	return ErrInvalidArgument
}

func supportedSecurityRulePeerType(peerType string) bool {
	peerType = strings.ToLower(strings.TrimSpace(peerType))
	return peerType == "device" || peerType == "device_group"
}

func requireManagedSecurityGroupWithNetwork(
	ctx context.Context,
	networks repository.NetworkRepository,
	securityGroupID string,
) (model.SecurityGroup, error) {
	item, err := requireManagedSecurityGroup(ctx, networks, securityGroupID)
	if err != nil {
		return model.SecurityGroup{}, err
	}
	if _, err := requireManagedNetwork(ctx, networks, item.NetworkID); err != nil {
		return model.SecurityGroup{}, err
	}
	return item, nil
}

func requireManagedSecurityRuleWithNetwork(
	ctx context.Context,
	networks repository.NetworkRepository,
	ruleID string,
) (model.SecurityRule, model.SecurityGroup, error) {
	rule, err := requireManagedSecurityRule(ctx, networks, ruleID)
	if err != nil {
		return model.SecurityRule{}, model.SecurityGroup{}, err
	}
	group, err := requireManagedSecurityGroupWithNetwork(ctx, networks, rule.SecurityGroupID)
	if err != nil {
		return model.SecurityRule{}, model.SecurityGroup{}, err
	}
	return rule, group, nil
}
