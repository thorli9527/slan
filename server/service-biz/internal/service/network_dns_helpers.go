package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

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
