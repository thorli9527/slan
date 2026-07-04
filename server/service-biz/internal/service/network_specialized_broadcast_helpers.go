package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func publishDNSChanged(
	ctx context.Context,
	networks repository.NetworkRepository,
	broadcaster networkBroadcastPublisher,
	nowFn func() time.Time,
	networkID string,
	version int64,
	reason string,
) error {
	if broadcaster == nil {
		return nil
	}
	zones, err := networks.ListDNSZones(ctx, networkID)
	if err != nil {
		return err
	}
	records, err := networks.ListDNSRecords(ctx, networkID)
	if err != nil {
		return err
	}
	return broadcaster.PublishDNSChanged(ctx, networkBroadcastDNSChanged{
		NetworkID:  strings.TrimSpace(networkID),
		Version:    version,
		Reason:     strings.TrimSpace(reason),
		Zones:      zones,
		Records:    records,
		OccurredAt: currentTime(nowFn),
	})
}

func publishACLChanged(
	ctx context.Context,
	networks repository.NetworkRepository,
	broadcaster networkBroadcastPublisher,
	nowFn func() time.Time,
	networkID string,
	version int64,
	reason string,
) error {
	if broadcaster == nil {
		return nil
	}
	securityGroups, err := networks.ListSecurityGroups(ctx, networkID)
	if err != nil {
		return err
	}
	securityRules := make([]model.SecurityRule, 0)
	for _, group := range securityGroups {
		items, err := networks.ListSecurityRules(ctx, group.SecurityGroupID)
		if err != nil {
			return err
		}
		securityRules = append(securityRules, items...)
	}
	publicMappings, err := networks.ListPublicMappings(ctx, networkID)
	if err != nil {
		return err
	}
	return broadcaster.PublishACLChanged(ctx, networkBroadcastACLChanged{
		NetworkID:      strings.TrimSpace(networkID),
		Version:        version,
		Reason:         strings.TrimSpace(reason),
		SecurityGroups: securityGroups,
		SecurityRules:  securityRules,
		PublicMappings: publicMappings,
		OccurredAt:     currentTime(nowFn),
	})
}
