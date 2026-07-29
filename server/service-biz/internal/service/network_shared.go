package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type NetworkCoreService struct {
	Users              repository.UserRepository
	Devices            repository.DeviceRepository
	Networks           repository.NetworkRepository
	Ops                repository.OpsRepository
	EventPublisher     NetworkEventPublisher
	VersionPushTracker *NetworkVersionPushTracker
	NewNetworkID       func() string
	Now                func() time.Time
}

type NetworkInviteService struct {
	Users           repository.UserRepository
	Devices         repository.DeviceRepository
	Relations       repository.DeviceRelationRepository
	Networks        repository.NetworkRepository
	EventPublisher  NetworkEventPublisher
	DevicePublisher DeviceControlPublisher
	NewInviteID     func() string
	Now             func() time.Time
}

type NetworkDNSService struct {
	Users          repository.UserRepository
	Devices        repository.DeviceRepository
	Networks       repository.NetworkRepository
	Ops            repository.OpsRepository
	EventPublisher NetworkEventPublisher
	NewDNSZoneID   func() string
	NewDNSRecordID func() string
	Now            func() time.Time
}

type NetworkAccessService struct {
	Users              repository.UserRepository
	Devices            repository.DeviceRepository
	Networks           repository.NetworkRepository
	Ops                repository.OpsRepository
	EventPublisher     NetworkEventPublisher
	NewSecurityGroupID func() string
	NewSecurityRuleID  func() string
	Now                func() time.Time
}

type NetworkRuntimeService struct {
	Devices   repository.DeviceRepository
	Networks  repository.NetworkRepository
	Ops       repository.OpsRepository
	NewSessID func(string) string
	Now       func() time.Time
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

func newManagedInviteID(next func() string) string {
	return generatedID(next, "invite")
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

func requireNetworkUser(ctx context.Context, users repository.UserRepository, userID string) (model.User, error) {
	user, ok, err := users.GetUser(ctx, userID)
	if err != nil {
		return model.User{}, err
	}
	if !ok {
		return model.User{}, ErrNotFound
	}
	return user, nil
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

func requireOwnedManagedDevice(ctx context.Context, users repository.UserRepository, devices repository.DeviceRepository, actorUserID, deviceID string) (model.Device, error) {
	device, err := requireManagedDevice(ctx, devices, deviceID)
	if err != nil {
		return model.Device{}, err
	}
	if actorUserID == "" {
		return device, nil
	}
	if _, err := requireNetworkUser(ctx, users, actorUserID); err != nil {
		return model.Device{}, err
	}
	if device.OwnerID != actorUserID {
		return model.Device{}, ErrForbidden
	}
	return device, nil
}

func requireOwnedManagedNetwork(ctx context.Context, users repository.UserRepository, networks repository.NetworkRepository, actorUserID, networkID string) (model.Network, error) {
	network, err := requireManagedNetwork(ctx, networks, networkID)
	if err != nil {
		return model.Network{}, err
	}
	if actorUserID == "" {
		return network, nil
	}
	if _, err := requireNetworkUser(ctx, users, actorUserID); err != nil {
		return model.Network{}, err
	}
	if network.OwnerID != actorUserID {
		return model.Network{}, ErrForbidden
	}
	return network, nil
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
