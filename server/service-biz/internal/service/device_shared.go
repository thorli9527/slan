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
	Catalog DeviceCatalogUseCase
	Runtime DeviceRuntimeAccessUseCase
}

type deviceCoreDependencies struct {
	Devices        repository.DeviceRepository
	Networks       repository.NetworkRepository
	MQTT           mqttkit.Config
	EventPublisher NetworkEventPublisher
	Now            func() time.Time
}

type DeviceCatalogService struct {
	deviceCoreDependencies
}

type DeviceRuntimeAccessService struct {
	deviceCoreDependencies
}

func NewDeviceCoreService(
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	mqtt mqttkit.Config,
	now func() time.Time,
) DeviceCoreService {
	deps := deviceCoreDependencies{
		Devices:        devices,
		Networks:       networks,
		MQTT:           mqtt,
		EventPublisher: NewNetworkEventPublisher(mqtt),
		Now:            now,
	}
	return DeviceCoreService{
		Catalog: DeviceCatalogService{deviceCoreDependencies: deps},
		Runtime: DeviceRuntimeAccessService{deviceCoreDependencies: deps},
	}
}

type DeviceGroupService struct {
	Devices         repository.DeviceRepository
	Networks        repository.NetworkRepository
	NetworkGroups   repository.NetworkDeviceGroupRepository
	EventPublisher  NetworkEventPublisher
	DevicePublisher DeviceControlPublisher
	Now             func() time.Time
}

type DeviceSessionService struct {
	Devices     repository.DeviceRepository
	Credentials repository.DeviceCredentialRepository
	Audit       repository.AuditRepository
	Networks    repository.NetworkRepository
	MQTT        mqttkit.Config
	NewSessID   func(string) string
	Now         func() time.Time
}

func (s DeviceCoreService) GetDeviceProfile(ctx context.Context, deviceID string) (DeviceProfileView, error) {
	return s.Catalog.GetDeviceProfile(ctx, deviceID)
}

func (s DeviceCoreService) RenewDevice(ctx context.Context, deviceID string) (DeviceProfileView, error) {
	return s.Runtime.RenewDevice(ctx, deviceID)
}

func (s DeviceCoreService) UpdateDeviceRuntime(ctx context.Context, input UpdateDeviceRuntimeInput) (DeviceProfileView, error) {
	return s.Runtime.UpdateDeviceRuntime(ctx, input)
}

func (s DeviceCoreService) DeviceNetworkConfigs(ctx context.Context, deviceID string) ([]NetworkSummaryView, error) {
	return s.Runtime.DeviceNetworkConfigs(ctx, deviceID)
}

func (s DeviceCoreService) DeviceMQTTCredential(ctx context.Context, deviceID, credentialID string, expiresAt int64) (*mqttkit.Credential, error) {
	return s.Runtime.DeviceMQTTCredential(ctx, deviceID, credentialID, expiresAt)
}

func (s DeviceCoreService) DeviceMQTTProfile(ctx context.Context, deviceID, credentialID string, expiresAt int64) (DeviceMQTTProfileView, error) {
	return s.Runtime.DeviceMQTTProfile(ctx, deviceID, credentialID, expiresAt)
}

func deviceNow(now func() time.Time) time.Time {
	return currentTime(now)
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

func allocateDeviceVirtualIP(devices repository.DeviceRepository) (string, error) {
	virtualIP := allocatedDeviceVirtualIP(newDeviceVirtualIPID(devices))
	if virtualIP == "" {
		return "", conflictError("device virtual IP pool exhausted")
	}
	return virtualIP, nil
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

func newDeviceMQTTProfile(cfg mqttkit.Config, now time.Time, deviceID, credentialID string, expiresAt int64, networks []model.Network) DeviceMQTTProfileView {
	credential := mqttkit.CredentialForDevice(cfg, deviceID, credentialID, now, expiresAt)
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
	accessTTL, err := deviceAccessTTL()
	if err != nil {
		return model.DeviceSession{}, err
	}
	refreshTTL, err := deviceRefreshTTL(sessionMode)
	if err != nil {
		return model.DeviceSession{}, err
	}
	access, err := randomHex(24)
	if err != nil {
		return model.DeviceSession{}, err
	}
	refresh, err := randomHex(24)
	if err != nil {
		return model.DeviceSession{}, err
	}
	return model.DeviceSession{
		SessionID:     newDeviceSessionID(next),
		DeviceID:      deviceID,
		AccessToken:   access,
		RefreshToken:  refresh,
		Status:        tokenStatusActive,
		SessionMode:   normalizedSessionMode(sessionMode),
		ExpiresAt:     now.Add(accessTTL).Unix(),
		RefreshExpiry: now.Add(refreshTTL).Unix(),
		CreatedAt:     now.Unix(),
		UpdatedAt:     now.Unix(),
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
