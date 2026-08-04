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
	default:
		return invalidArgumentError("DNS record type must be A, AAAA, or CNAME")
	}
	return nil
}

func validateManagedDNSRecordZone(
	ctx context.Context,
	networks repository.NetworkRepository,
	networkID string,
	zoneID string,
) error {
	zoneID = strings.TrimSpace(zoneID)
	if zoneID == "" {
		return invalidArgumentError("DNS zone is required")
	}
	zone, err := requireManagedDNSZone(ctx, networks, zoneID)
	if err != nil {
		return err
	}
	if zone.NetworkID != strings.TrimSpace(networkID) {
		return invalidArgumentError("DNS zone must belong to the current network")
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

func requireManagedDNSZoneWithNetwork(
	ctx context.Context,
	networks repository.NetworkRepository,
	zoneID string,
) (model.DNSZone, error) {
	item, err := requireManagedDNSZone(ctx, networks, zoneID)
	if err != nil {
		return model.DNSZone{}, err
	}
	if _, err := requireManagedNetwork(ctx, networks, item.NetworkID); err != nil {
		return model.DNSZone{}, err
	}
	return item, nil
}

func requireManagedDNSRecordWithNetwork(
	ctx context.Context,
	networks repository.NetworkRepository,
	recordID string,
) (model.DNSRecord, error) {
	item, err := requireManagedDNSRecord(ctx, networks, recordID)
	if err != nil {
		return model.DNSRecord{}, err
	}
	if _, err := requireManagedNetwork(ctx, networks, item.NetworkID); err != nil {
		return model.DNSRecord{}, err
	}
	return item, nil
}
