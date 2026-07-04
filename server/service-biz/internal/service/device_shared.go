package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

type DeviceCoreService struct {
	Catalog      DeviceCatalogUseCase
	Provisioning DeviceProvisioningUseCase
	Runtime      DeviceRuntimeAccessUseCase
}

type deviceCoreDependencies struct {
	Users       repository.UserRepository
	Devices     repository.DeviceRepository
	Networks    repository.NetworkRepository
	MQTT        mqttkit.Config
	Broadcaster networkBroadcastPublisher
	NewDeviceID func() string
	Now         func() time.Time
}

type DeviceCatalogService struct {
	deviceCoreDependencies
}

type DeviceProvisioningService struct {
	deviceCoreDependencies
}

type DeviceRuntimeAccessService struct {
	deviceCoreDependencies
}

type DeviceBootstrapService struct {
	Keys     DeviceBootstrapKeyUseCase
	Sessions DeviceBootstrapSessionUseCase
}

type deviceBootstrapDependencies struct {
	Users     repository.UserRepository
	Devices   repository.DeviceRepository
	Networks  repository.NetworkRepository
	MQTT      mqttkit.Config
	NewSessID func(string) string
	Now       func() time.Time
}

type DeviceBootstrapKeyService struct {
	deviceBootstrapDependencies
}

type DeviceBootstrapSessionService struct {
	deviceBootstrapDependencies
}

func NewDeviceBootstrapService(
	users repository.UserRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	mqtt mqttkit.Config,
	newSessionID func(string) string,
	now func() time.Time,
) DeviceBootstrapService {
	deps := deviceBootstrapDependencies{
		Users:     users,
		Devices:   devices,
		Networks:  networks,
		MQTT:      mqtt,
		NewSessID: newSessionID,
		Now:       now,
	}
	return DeviceBootstrapService{
		Keys:     DeviceBootstrapKeyService{deviceBootstrapDependencies: deps},
		Sessions: DeviceBootstrapSessionService{deviceBootstrapDependencies: deps},
	}
}

func NewDeviceCoreService(
	users repository.UserRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	mqtt mqttkit.Config,
	newDeviceID func() string,
	now func() time.Time,
) DeviceCoreService {
	deps := deviceCoreDependencies{
		Users:       users,
		Devices:     devices,
		Networks:    networks,
		MQTT:        mqtt,
		Broadcaster: newNetworkBroadcastPublisher(mqtt),
		NewDeviceID: newDeviceID,
		Now:         now,
	}
	return DeviceCoreService{
		Catalog:      DeviceCatalogService{deviceCoreDependencies: deps},
		Provisioning: DeviceProvisioningService{deviceCoreDependencies: deps},
		Runtime:      DeviceRuntimeAccessService{deviceCoreDependencies: deps},
	}
}

type DeviceGroupService struct {
	Users   repository.UserRepository
	Devices repository.DeviceRepository
	Now     func() time.Time
}

type DeviceSessionService struct {
	Devices   repository.DeviceRepository
	Networks  repository.NetworkRepository
	Users     repository.UserRepository
	MQTT      mqttkit.Config
	NewSessID func(string) string
	Now       func() time.Time
}

func (s DeviceBootstrapService) CreateDeviceBootstrapKey(ctx context.Context, input CreateDeviceBootstrapKeyInput) (DeviceBootstrapKeyView, error) {
	return s.Keys.CreateDeviceBootstrapKey(ctx, input)
}

func (s DeviceBootstrapService) ListDeviceBootstrapKeys(ctx context.Context, userID string) ([]DeviceBootstrapKeyView, error) {
	return s.Keys.ListDeviceBootstrapKeys(ctx, userID)
}

func (s DeviceBootstrapService) RevokeDeviceBootstrapKey(ctx context.Context, input RevokeDeviceBootstrapKeyInput) (DeviceBootstrapKeyView, error) {
	return s.Keys.RevokeDeviceBootstrapKey(ctx, input)
}

func (s DeviceBootstrapService) BootstrapDeviceSession(ctx context.Context, input BootstrapDeviceSessionInput) (DeviceSessionBootstrapView, error) {
	return s.Sessions.BootstrapDeviceSession(ctx, input)
}

func (s DeviceCoreService) ListDevices(ctx context.Context, ownerID string) ([]DeviceView, error) {
	return s.Catalog.ListDevices(ctx, ownerID)
}

func (s DeviceCoreService) ListVisibleDevices(ctx context.Context, ownerID string) ([]DeviceView, error) {
	return s.Catalog.ListVisibleDevices(ctx, ownerID)
}

func (s DeviceCoreService) ListDeviceProfiles(ctx context.Context, ownerID string) ([]DeviceProfileView, error) {
	return s.Catalog.ListDeviceProfiles(ctx, ownerID)
}

func (s DeviceCoreService) ListVisibleDeviceProfiles(ctx context.Context, ownerID string) ([]DeviceProfileView, error) {
	return s.Catalog.ListVisibleDeviceProfiles(ctx, ownerID)
}

func (s DeviceCoreService) GetDevice(ctx context.Context, deviceID string) (DeviceView, error) {
	return s.Catalog.GetDevice(ctx, deviceID)
}

func (s DeviceCoreService) GetDeviceProfile(ctx context.Context, deviceID string) (DeviceProfileView, error) {
	return s.Catalog.GetDeviceProfile(ctx, deviceID)
}

func (s DeviceCoreService) RegisterDevice(ctx context.Context, input RegisterDeviceInput) (DeviceProfileView, error) {
	return s.Provisioning.RegisterDevice(ctx, input)
}

func (s DeviceCoreService) UpdateDeviceAlias(ctx context.Context, input UpdateDeviceAliasInput) (DeviceProfileView, error) {
	return s.Provisioning.UpdateDeviceAlias(ctx, input)
}

func (s DeviceCoreService) DeleteDevice(ctx context.Context, input DeleteDeviceInput) error {
	return s.Provisioning.DeleteDevice(ctx, input)
}

func (s DeviceCoreService) RenewDevice(ctx context.Context, deviceID string) (DeviceProfileView, error) {
	return s.Provisioning.RenewDevice(ctx, deviceID)
}

func (s DeviceCoreService) UpdateDeviceRuntime(ctx context.Context, input UpdateDeviceRuntimeInput) (DeviceProfileView, error) {
	return s.Runtime.UpdateDeviceRuntime(ctx, input)
}

func (s DeviceCoreService) DeviceNetworkConfigs(ctx context.Context, deviceID string) ([]NetworkSummaryView, error) {
	return s.Runtime.DeviceNetworkConfigs(ctx, deviceID)
}

func (s DeviceCoreService) DeviceMQTTCredential(ctx context.Context, deviceID string) (*mqttkit.Credential, error) {
	return s.Runtime.DeviceMQTTCredential(ctx, deviceID)
}

func (s DeviceCoreService) DeviceMQTTProfile(ctx context.Context, deviceID string) (DeviceMQTTProfileView, error) {
	return s.Runtime.DeviceMQTTProfile(ctx, deviceID)
}

func deviceNow(now func() time.Time) time.Time {
	return currentTime(now)
}

func newManagedDeviceID(next func() string) string {
	return generatedID(next, "device")
}

func newDeviceBootstrapKeyID(devices repository.DeviceRepository) string {
	return repositoryID[deviceBootstrapKeyIDProvider](devices, "dbk", func(provider deviceBootstrapKeyIDProvider) string {
		return provider.NewDeviceBootstrapKeyID()
	})
}

func newDeviceGroupID(devices repository.DeviceRepository) string {
	return repositoryID[deviceGroupIDProvider](devices, "dgrp", func(provider deviceGroupIDProvider) string {
		return provider.NewDeviceGroupID()
	})
}

func newDeviceVirtualIPID(devices repository.DeviceRepository) string {
	return repositoryID[deviceVirtualIPIDProvider](devices, "vip00000000000000000000000000000000", func(provider deviceVirtualIPIDProvider) string {
		return provider.NewDeviceVirtualIPID()
	})
}

func newDeviceSessionID(next func(string) string) string {
	return scopedID(next, "dsess")
}

func mqttTopicRoot(cfg mqttkit.Config) string {
	root := strings.Trim(strings.TrimSpace(cfg.TopicPrefix), "/")
	if root == "" {
		return "slan"
	}
	return root
}

func mqttDeviceTopicPrefix(cfg mqttkit.Config, deviceID string) string {
	return fmt.Sprintf("%s/devices/%s", mqttTopicRoot(cfg), strings.TrimSpace(deviceID))
}

func mqttPublishTopics(cfg mqttkit.Config, deviceID string, networks []model.Network) []string {
	topics := []string{
		mqttDeviceTopicPrefix(cfg, deviceID) + "/control/up",
		mqttDeviceTopicPrefix(cfg, deviceID) + "/control/ack",
		mqttDeviceTopicPrefix(cfg, deviceID) + "/heartbeat",
		mqttDeviceTopicPrefix(cfg, deviceID) + "/runtime",
		mqttDeviceTopicPrefix(cfg, deviceID) + "/runtime-state",
	}
	for _, network := range networks {
		if strings.TrimSpace(network.NetworkID) == "" {
			continue
		}
		topics = append(topics, fmt.Sprintf("%s/networks/%s/members/%s/state", mqttTopicRoot(cfg), network.NetworkID, deviceID))
	}
	return topics
}

func mqttSubscribeTopics(cfg mqttkit.Config, deviceID string, networks []model.Network) []string {
	topics := []string{mqttDeviceTopicPrefix(cfg, deviceID) + "/control/down"}
	for _, network := range networks {
		if strings.TrimSpace(network.NetworkID) == "" {
			continue
		}
		topics = append(topics, fmt.Sprintf("%s/networks/%s/broadcast", mqttTopicRoot(cfg), network.NetworkID))
	}
	return topics
}

func mqttNetworkIDs(networks []model.Network) []string {
	ids := make([]string, 0, len(networks))
	for _, network := range networks {
		if strings.TrimSpace(network.NetworkID) == "" {
			continue
		}
		ids = append(ids, network.NetworkID)
	}
	return ids
}

func newDeviceMQTTProfile(cfg mqttkit.Config, now time.Time, deviceID string, networks []model.Network) DeviceMQTTProfileView {
	credential := mqttkit.CredentialForDevice(cfg, deviceID, now)
	view := DeviceMQTTProfileView{
		Enabled:         cfg.Enabled,
		TopicPrefix:     mqttDeviceTopicPrefix(cfg, deviceID),
		PublishTopics:   mqttPublishTopics(cfg, deviceID, networks),
		SubscribeTopics: mqttSubscribeTopics(cfg, deviceID, networks),
		NetworkIDs:      mqttNetworkIDs(networks),
		Credential:      credential,
	}
	if credential != nil {
		view.BrokerURL = credential.BrokerURL
		view.ClientID = credential.ClientID
		view.Username = credential.Username
		view.Password = credential.Password
		view.ExpiresAt = credential.ExpiresAt
	}
	return view
}

func newManagedDeviceSession(now time.Time, next func(string) string, deviceID string, sessionMode string) (model.DeviceSession, error) {
	access, err := randomHex(24)
	if err != nil {
		return model.DeviceSession{}, err
	}
	refresh, err := randomHex(24)
	if err != nil {
		return model.DeviceSession{}, err
	}
	return model.DeviceSession{
		SessionID:    newDeviceSessionID(next),
		DeviceID:     deviceID,
		AccessToken:  access,
		RefreshToken: refresh,
		Status:       tokenStatusActive,
		SessionMode:  normalizedSessionMode(sessionMode),
		ExpiresAt:    now.Add(defaultDeviceAccessTTL).Unix(),
		RefreshExpiry: now.Add(deviceRefreshTTL(sessionMode)).Unix(),
		CreatedAt:    now.Unix(),
		UpdatedAt:    now.Unix(),
	}, nil
}

func requireDeviceNetwork(ctx context.Context, networks repository.NetworkRepository, networkID string) (model.Network, error) {
	networkID = normalizeNetworkID(networkID)
	if networkID == "" {
		return model.Network{}, ErrInvalidArgument
	}
	network, ok, err := networks.GetNetwork(ctx, networkID)
	if err != nil {
		return model.Network{}, err
	}
	if !ok {
		return model.Network{}, ErrNotFound
	}
	return network, nil
}

func getManagedDevice(ctx context.Context, devices repository.DeviceRepository, deviceID string) (model.Device, error) {
	deviceID = normalizeDeviceID(deviceID)
	if deviceID == "" {
		return model.Device{}, ErrInvalidArgument
	}
	item, ok, err := devices.GetDevice(ctx, deviceID)
	if err != nil {
		return model.Device{}, err
	}
	if !ok {
		return model.Device{}, ErrNotFound
	}
	return item, nil
}
