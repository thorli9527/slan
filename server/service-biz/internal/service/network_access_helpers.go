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
	actorUserID string,
	networkID string,
	peerType string,
	peerValue string,
) error {
	peerType, peerValue = normalizedSecurityPeer(peerType, peerValue, "")
	networkID = normalizeNetworkID(networkID)
	activeNetworkDevicesByID := map[string]model.NetworkDevice{}
	if networkID != "" && networks != nil {
		items, err := networks.ListNetworkDevices(ctx, networkID)
		if err != nil {
			return err
		}
		for _, item := range items {
			if !networkMemberActive(item) {
				continue
			}
			activeNetworkDevicesByID[item.DeviceID] = item
		}
	}
	switch peerType {
	case "", "cidr":
		return ErrInvalidArgument
	case "all":
		if peerValue == "" {
			return nil
		}
		if strings.EqualFold(peerValue, "all") || strings.EqualFold(peerValue, "any") || peerValue == "*" {
			return nil
		}
		return ErrInvalidArgument
	case "device":
		if peerValue == "" {
			return ErrInvalidArgument
		}
		if _, ok := activeNetworkDevicesByID[peerValue]; !ok {
			return ErrInvalidArgument
		}
		return nil
	case "user", "workspace":
		if peerValue == "" {
			return ErrInvalidArgument
		}
		return nil
	case "device_group":
		if peerValue == "" || devices == nil {
			return ErrInvalidArgument
		}
		group, ok, err := devices.GetDeviceGroup(ctx, peerValue)
		if err != nil {
			return err
		}
		if !ok || strings.TrimSpace(group.UserID) == "" || group.UserID != actorUserID {
			return ErrInvalidArgument
		}
		assignments, err := devices.ListDeviceGroupAssignments(ctx, actorUserID)
		if err != nil {
			return err
		}
		for _, assignment := range assignments {
			if _, ok := activeNetworkDevicesByID[assignment.DeviceID]; !ok {
				continue
			}
			for _, groupID := range assignment.GroupIDs {
				if strings.TrimSpace(groupID) == peerValue {
					return nil
				}
			}
		}
		return ErrInvalidArgument
	default:
		return ErrInvalidArgument
	}
}

func requireOwnedManagedPublicMapping(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	actorUserID string,
	mappingID string,
) (model.PublicMapping, error) {
	item, err := requireManagedPublicMapping(ctx, networks, mappingID)
	if err != nil {
		return model.PublicMapping{}, err
	}
	if _, err := requireOwnedManagedNetwork(ctx, users, networks, actorUserID, item.NetworkID); err != nil {
		return model.PublicMapping{}, err
	}
	return item, nil
}

func requireOwnedManagedSecurityGroup(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	actorUserID string,
	securityGroupID string,
) (model.SecurityGroup, error) {
	item, err := requireManagedSecurityGroup(ctx, networks, securityGroupID)
	if err != nil {
		return model.SecurityGroup{}, err
	}
	if _, err := requireOwnedManagedNetwork(ctx, users, networks, actorUserID, item.NetworkID); err != nil {
		return model.SecurityGroup{}, err
	}
	return item, nil
}

func requireOwnedManagedSecurityRule(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	actorUserID string,
	ruleID string,
) (model.SecurityRule, model.SecurityGroup, error) {
	rule, err := requireManagedSecurityRule(ctx, networks, ruleID)
	if err != nil {
		return model.SecurityRule{}, model.SecurityGroup{}, err
	}
	group, err := requireOwnedManagedSecurityGroup(ctx, users, networks, actorUserID, rule.SecurityGroupID)
	if err != nil {
		return model.SecurityRule{}, model.SecurityGroup{}, err
	}
	return rule, group, nil
}
