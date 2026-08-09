package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type NetworkCoreService struct {
	Devices            repository.DeviceRepository
	Networks           repository.NetworkRepository
	RuntimeNodes       repository.RuntimeNodeRepository
	EventPublisher     NetworkEventPublisher
	VersionPushTracker *NetworkVersionPushTracker
	NewNetworkID       func() string
	Now                func() time.Time
}

type NetworkDNSService struct {
	Devices        repository.DeviceRepository
	Networks       repository.NetworkRepository
	RuntimeNodes   repository.RuntimeNodeRepository
	EventPublisher NetworkEventPublisher
	NewDNSZoneID   func() string
	NewDNSRecordID func() string
	Now            func() time.Time
}

type NetworkAccessService struct {
	Devices            repository.DeviceRepository
	Networks           repository.NetworkRepository
	RuntimeNodes       repository.RuntimeNodeRepository
	EventPublisher     NetworkEventPublisher
	NewSecurityGroupID func() string
	NewSecurityRuleID  func() string
	Now                func() time.Time
}

type NetworkRuntimeService struct {
	Devices      repository.DeviceRepository
	Networks     repository.NetworkRepository
	RuntimeNodes repository.RuntimeNodeRepository
	LocateIP     func(string) (DeviceLocation, bool)
	NewSessID    func(string) string
	Now          func() time.Time
}

func networkNow(now func() time.Time) time.Time {
	return currentTime(now)
}

func newNetworkSessionID(next func(string) string, prefix string) string {
	return scopedID(next, prefix)
}

func newManagedNetworkID(next func() string) string {
	return generatedID(next, "net")
}

func newManagedDNSZoneID(networks repository.NetworkRepository, next func() string) string {
	if next != nil {
		return generatedID(next, "zone")
	}
	return repositoryID[dnsZoneIDProvider](networks, "zone", func(provider dnsZoneIDProvider) string {
		return provider.NewDNSZoneID()
	})
}

func newManagedDNSRecordID(networks repository.NetworkRepository, next func() string) string {
	if next != nil {
		return generatedID(next, "rec")
	}
	return repositoryID[dnsRecordIDProvider](networks, "rec", func(provider dnsRecordIDProvider) string {
		return provider.NewDNSRecordID()
	})
}

func newManagedSecurityGroupID(networks repository.NetworkRepository, next func() string) string {
	if next != nil {
		return generatedID(next, "sg")
	}
	return repositoryID[securityGroupIDProvider](networks, "sg", func(provider securityGroupIDProvider) string {
		return provider.NewSecurityGroupID()
	})
}

func newManagedSecurityRuleID(networks repository.NetworkRepository, next func() string) string {
	if next != nil {
		return generatedID(next, "sgr")
	}
	return repositoryID[securityRuleIDProvider](networks, "sgr", func(provider securityRuleIDProvider) string {
		return provider.NewSecurityRuleID()
	})
}

func requireManagedNetwork(ctx context.Context, networks repository.NetworkRepository, networkID string) (model.Network, error) {
	network, ok, err := networks.GetNetwork(ctx, networkID)
	if err != nil {
		return model.Network{}, err
	}
	if !ok {
		return model.Network{}, ErrNotFound
	}
	return network, nil
}

func requireManagedDevice(ctx context.Context, devices repository.DeviceRepository, deviceID string) (model.Device, error) {
	device, ok, err := devices.GetDevice(ctx, deviceID)
	if err != nil {
		return model.Device{}, err
	}
	if !ok {
		return model.Device{}, ErrNotFound
	}
	return device, nil
}

func requireManagedDNSRecord(ctx context.Context, networks repository.NetworkRepository, recordID string) (model.DNSRecord, error) {
	item, ok, err := networks.GetDNSRecord(ctx, recordID)
	if err != nil {
		return model.DNSRecord{}, err
	}
	if !ok {
		return model.DNSRecord{}, ErrNotFound
	}
	return item, nil
}

func requireManagedSecurityGroup(ctx context.Context, networks repository.NetworkRepository, securityGroupID string) (model.SecurityGroup, error) {
	item, ok, err := networks.GetSecurityGroup(ctx, securityGroupID)
	if err != nil {
		return model.SecurityGroup{}, err
	}
	if !ok {
		return model.SecurityGroup{}, ErrNotFound
	}
	return item, nil
}

func requireManagedSecurityRule(ctx context.Context, networks repository.NetworkRepository, ruleID string) (model.SecurityRule, error) {
	item, ok, err := networks.GetSecurityRule(ctx, ruleID)
	if err != nil {
		return model.SecurityRule{}, err
	}
	if !ok {
		return model.SecurityRule{}, ErrNotFound
	}
	return item, nil
}
