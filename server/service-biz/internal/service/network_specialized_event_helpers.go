package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func publishDNSChanged(
	ctx context.Context,
	networks repository.NetworkRepository,
	eventPublisher NetworkEventPublisher,
	nowFn func() time.Time,
	networkID string,
	version int64,
	reason string,
) error {
	if eventPublisher == nil {
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
	dnsConfig := BuildNetworkDNSConfigView(NetworkConfigView{
		DNSZones: dnsZoneViews(zones),
	})
	occurredAt := currentTime(nowFn)
	_ = zones
	return publishNetworkEvent(
		ctx,
		eventPublisher,
		NetworkEventDNSChanged,
		networkID,
		uint64(version),
		occurredAt.UnixMilli(),
		NetworkEventDNSChangedPayload{
			Config: NetworkEventDNSConfigView{
				Servers:                   append([]string(nil), dnsConfig.Servers...),
				SearchDomains:             append([]string(nil), dnsConfig.SearchDomains...),
				SplitDomains:              append([]string(nil), dnsConfig.SplitDomains...),
				FallbackToSystemResolvers: dnsConfig.FallbackToSystemResolvers,
			},
			Zones:   networkEventDNSZones(zones),
			Records: networkEventDNSRecords(records, zones),
		},
	)
}

func publishACLChanged(
	ctx context.Context,
	networks repository.NetworkRepository,
	eventPublisher NetworkEventPublisher,
	nowFn func() time.Time,
	networkID string,
	version int64,
	reason string,
) error {
	if eventPublisher == nil {
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
		for _, item := range items {
			if supportedSecurityRulePeerType(item.PeerType) {
				securityRules = append(securityRules, item)
			}
		}
	}
	occurredAt := currentTime(nowFn)
	_ = securityGroups
	return publishNetworkEvent(
		ctx,
		eventPublisher,
		NetworkEventACLChanged,
		networkID,
		uint64(version),
		occurredAt.UnixMilli(),
		NetworkEventACLChangedPayload{
			Rules: networkEventACLRules(securityRules),
		},
	)
}
