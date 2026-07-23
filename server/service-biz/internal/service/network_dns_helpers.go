package service

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func validateManagedDNSRecordTarget(
	ctx context.Context,
	networks repository.NetworkRepository,
	networkID string,
	recordType string,
	value string,
) error {
	switch strings.ToUpper(strings.TrimSpace(recordType)) {
	case "A", "AAAA":
		deviceID := strings.TrimSpace(value)
		if deviceID == "" {
			return invalidArgumentError("DNS address record target device is required")
		}
		member, ok, err := networks.GetNetworkDevice(ctx, strings.TrimSpace(networkID), deviceID)
		if err != nil {
			return err
		}
		if !ok || !networkMemberActive(member) {
			return invalidArgumentError("DNS address record target must be an active device in the current network")
		}
	case "CNAME":
		if strings.TrimSpace(value) == "" {
			return invalidArgumentError("DNS CNAME target is required")
		}
	}
	return nil
}

func requireManagedDNSZone(ctx context.Context, networks repository.NetworkRepository, zoneID string) (model.DNSZone, error) {
	item, ok, err := networks.GetDNSZone(ctx, zoneID)
	if err != nil {
		return model.DNSZone{}, err
	}
	if !ok {
		return model.DNSZone{}, ErrNotFound
	}
	return item, nil
}

func requireOwnedManagedDNSZone(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	actorUserID string,
	zoneID string,
) (model.DNSZone, error) {
	item, err := requireManagedDNSZone(ctx, networks, zoneID)
	if err != nil {
		return model.DNSZone{}, err
	}
	if _, err := requireOwnedManagedNetwork(ctx, users, networks, actorUserID, item.NetworkID); err != nil {
		return model.DNSZone{}, err
	}
	return item, nil
}

func requireOwnedManagedDNSRecord(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	actorUserID string,
	recordID string,
) (model.DNSRecord, error) {
	item, err := requireManagedDNSRecord(ctx, networks, recordID)
	if err != nil {
		return model.DNSRecord{}, err
	}
	if _, err := requireOwnedManagedNetwork(ctx, users, networks, actorUserID, item.NetworkID); err != nil {
		return model.DNSRecord{}, err
	}
	return item, nil
}
