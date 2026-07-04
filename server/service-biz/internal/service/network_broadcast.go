package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type networkBroadcastPublisher interface {
	PublishNetworkMemberStateChanged(context.Context, networkBroadcastMemberState) error
	PublishNetworkConfigChanged(context.Context, networkBroadcastConfigChanged) error
	PublishNetworkSnapshot(context.Context, networkBroadcastSnapshot) error
	PublishDNSChanged(context.Context, networkBroadcastDNSChanged) error
	PublishACLChanged(context.Context, networkBroadcastACLChanged) error
	PublishNetworkMemberChanged(context.Context, networkBroadcastMemberChanged) error
	PublishDevicePresenceChanged(context.Context, networkBroadcastDevicePresenceChanged) error
}

type networkBroadcastMemberState struct {
	Network  model.Network
	Device   model.Device
	Member   model.NetworkDevice
	Members  []model.NetworkDevice
	Now      time.Time
	Online   bool
	PrefixLen int
}

type networkBroadcastConfigChanged struct {
	NetworkID  string
	Version    int64
	Reason     string
	OccurredAt time.Time
}

type networkBroadcastSnapshot struct {
	NetworkID  string
	Version    int64
	Reason     string
	Snapshot   map[string]any
	OccurredAt time.Time
}

type networkBroadcastDNSChanged struct {
	NetworkID  string
	Version    int64
	Reason     string
	Zones      []model.DNSZone
	Records    []model.DNSRecord
	OccurredAt time.Time
}

type networkBroadcastACLChanged struct {
	NetworkID       string
	Version         int64
	Reason          string
	SecurityGroups  []model.SecurityGroup
	SecurityRules   []model.SecurityRule
	PublicMappings  []model.PublicMapping
	OccurredAt      time.Time
}

type networkBroadcastMemberChanged struct {
	NetworkID   string
	DeviceID    string
	Action      string
	Member      model.NetworkDevice
	Version     int64
	Reason      string
	OccurredAt  time.Time
}

type networkBroadcastDevicePresenceChanged struct {
	NetworkID        string
	DeviceID         string
	PresenceStatus   model.DevicePresenceStatus
	MQTTConnected    bool
	ActivePath       string
	LastSeenAt       int64
	LastRuntimeStateAt int64
	OccurredAt       time.Time
}

type mqttNetworkBroadcastPublisher struct {
	config mqttkit.Config
}

func (p mqttNetworkBroadcastPublisher) publishNetworkBroadcast(
	ctx context.Context,
	occurredAt time.Time,
	networkID string,
	encodeErrPrefix string,
	payloadBody map[string]any,
	publishErrPrefix string,
) error {
	credential := mqttkit.CredentialForServer(p.config, occurredAt)
	if credential == nil {
		return nil
	}
	brokerURL, err := mqttBrokerURLForClient(credential.BrokerURL)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(payloadBody)
	if err != nil {
		return fmt.Errorf("%s: %w", encodeErrPrefix, err)
	}
	topic := fmt.Sprintf("%s/networks/%s/broadcast", mqttTopicRoot(p.config), strings.TrimSpace(networkID))
	return p.publishWithRetry(ctx, brokerURL, credential, topic, payload, publishErrPrefix)
}

func (p mqttNetworkBroadcastPublisher) publishWithRetry(
	ctx context.Context,
	brokerURL string,
	credential *mqttkit.Credential,
	topic string,
	payload []byte,
	publishErrPrefix string,
) error {
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if err := p.publishOnce(ctx, brokerURL, credential, topic, payload, publishErrPrefix, attempt); err != nil {
			lastErr = err
			if ctx.Err() != nil || attempt >= 2 {
				break
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		return nil
	}
	return lastErr
}

func (p mqttNetworkBroadcastPublisher) publishOnce(
	ctx context.Context,
	brokerURL string,
	credential *mqttkit.Credential,
	topic string,
	payload []byte,
	publishErrPrefix string,
	attempt int,
) error {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(serverMQTTClientID(credential.ClientID, "network-broadcast", attempt)).
		SetUsername(credential.Username).
		SetPassword(credential.Password).
		SetConnectTimeout(3 * time.Second).
		SetWriteTimeout(3 * time.Second).
		SetOrderMatters(false).
		SetAutoReconnect(false).
		SetCleanSession(true)
	client := mqtt.NewClient(opts)
	connectToken := client.Connect()
	if ok := connectToken.WaitTimeout(4 * time.Second); !ok {
		return fmt.Errorf("connect mqtt broker timeout")
	}
	if err := connectToken.Error(); err != nil {
		return fmt.Errorf("connect mqtt broker: %w", err)
	}
	defer client.Disconnect(250)
	publishToken := client.Publish(topic, 1, false, payload)
	if deadline, ok := ctx.Deadline(); ok {
		if !publishToken.WaitTimeout(time.Until(deadline)) {
			return context.DeadlineExceeded
		}
	} else if !publishToken.WaitTimeout(4 * time.Second) {
		return fmt.Errorf("%s timeout", publishErrPrefix)
	}
	if err := publishToken.Error(); err != nil {
		return fmt.Errorf("%s: %w", publishErrPrefix, err)
	}
	return nil
}

func newNetworkBroadcastPublisher(cfg mqttkit.Config) networkBroadcastPublisher {
	if !cfg.Enabled || strings.TrimSpace(cfg.BrokerURL) == "" {
		return nil
	}
	return mqttNetworkBroadcastPublisher{config: cfg}
}

func NewNetworkBroadcastPublisher(cfg mqttkit.Config) networkBroadcastPublisher {
	return newNetworkBroadcastPublisher(cfg)
}

func (p mqttNetworkBroadcastPublisher) PublishNetworkMemberStateChanged(
	ctx context.Context,
	state networkBroadcastMemberState,
) error {
	return p.publishNetworkBroadcast(
		ctx,
		state.Now,
		state.Network.NetworkID,
		"encode network member state broadcast",
		map[string]any{
		"type": "network_member_changed",
		"payload": map[string]any{
			"networkId": state.Network.NetworkID,
			"deviceId": state.Device.DeviceID,
			"virtualIp": state.Device.VirtualIP,
			"prefixLen": state.PrefixLen,
			"online": state.Online,
			"onlineDevices": networkBroadcastOnlineDevices(state.Members),
			"reportedAtMs": state.Now.UnixMilli(),
		},
		},
		"publish network member state",
	)
}

func (p mqttNetworkBroadcastPublisher) PublishNetworkConfigChanged(
	ctx context.Context,
	state networkBroadcastConfigChanged,
) error {
	return p.publishNetworkBroadcast(
		ctx,
		state.OccurredAt,
		state.NetworkID,
		"encode network config changed broadcast",
		map[string]any{
		"type": "network_config_changed",
		"payload": map[string]any{
			"networkId":    strings.TrimSpace(state.NetworkID),
			"configVersion": state.Version,
			"reason":       strings.TrimSpace(state.Reason),
			"reportedAtMs": state.OccurredAt.UnixMilli(),
		},
		},
		"publish network config changed",
	)
}

func (p mqttNetworkBroadcastPublisher) PublishNetworkSnapshot(
	ctx context.Context,
	state networkBroadcastSnapshot,
) error {
	return p.publishNetworkBroadcast(
		ctx,
		state.OccurredAt,
		state.NetworkID,
		"encode network snapshot broadcast",
		map[string]any{
		"type": "network_snapshot",
		"payload": map[string]any{
			"networkId":     strings.TrimSpace(state.NetworkID),
			"configVersion": state.Version,
			"reason":        strings.TrimSpace(state.Reason),
			"snapshot":      state.Snapshot,
			"reportedAtMs":  state.OccurredAt.UnixMilli(),
		},
		},
		"publish network snapshot",
	)
}

func (p mqttNetworkBroadcastPublisher) PublishDNSChanged(
	ctx context.Context,
	state networkBroadcastDNSChanged,
) error {
	return p.publishNetworkBroadcast(
		ctx,
		state.OccurredAt,
		state.NetworkID,
		"encode dns changed broadcast",
		map[string]any{
		"type": "dns_changed",
		"payload": map[string]any{
			"networkId":     strings.TrimSpace(state.NetworkID),
			"configVersion": state.Version,
			"reason":        strings.TrimSpace(state.Reason),
			"zones":         state.Zones,
			"records":       state.Records,
			"reportedAtMs":  state.OccurredAt.UnixMilli(),
		},
		},
		"publish dns changed",
	)
}

func (p mqttNetworkBroadcastPublisher) PublishACLChanged(
	ctx context.Context,
	state networkBroadcastACLChanged,
) error {
	return p.publishNetworkBroadcast(
		ctx,
		state.OccurredAt,
		state.NetworkID,
		"encode acl changed broadcast",
		map[string]any{
		"type": "acl_changed",
		"payload": map[string]any{
			"networkId":      strings.TrimSpace(state.NetworkID),
			"configVersion":  state.Version,
			"reason":         strings.TrimSpace(state.Reason),
			"securityGroups": state.SecurityGroups,
			"securityRules":  state.SecurityRules,
			"publicMappings": state.PublicMappings,
			"reportedAtMs":   state.OccurredAt.UnixMilli(),
		},
		},
		"publish acl changed",
	)
}

func (p mqttNetworkBroadcastPublisher) PublishNetworkMemberChanged(
	ctx context.Context,
	state networkBroadcastMemberChanged,
) error {
	return p.publishNetworkBroadcast(
		ctx,
		state.OccurredAt,
		state.NetworkID,
		"encode network member changed broadcast",
		map[string]any{
		"type": "network_member_changed",
		"payload": map[string]any{
			"networkId":     strings.TrimSpace(state.NetworkID),
			"deviceId":      strings.TrimSpace(state.DeviceID),
			"action":        strings.TrimSpace(state.Action),
			"member":        state.Member,
			"configVersion": state.Version,
			"reason":        strings.TrimSpace(state.Reason),
			"reportedAtMs":  state.OccurredAt.UnixMilli(),
		},
		},
		"publish network member changed",
	)
}

func (p mqttNetworkBroadcastPublisher) PublishDevicePresenceChanged(
	ctx context.Context,
	state networkBroadcastDevicePresenceChanged,
) error {
	return p.publishNetworkBroadcast(
		ctx,
		state.OccurredAt,
		state.NetworkID,
		"encode device presence changed broadcast",
		map[string]any{
		"type": "device_network_enabled",
		"payload": map[string]any{
			"networkId":         strings.TrimSpace(state.NetworkID),
			"deviceId":          strings.TrimSpace(state.DeviceID),
			"presenceStatus":    state.PresenceStatus,
			"mqttConnected":     state.MQTTConnected,
			"activePath":        strings.TrimSpace(state.ActivePath),
			"lastSeenAt":        state.LastSeenAt,
			"lastRuntimeStateAt": state.LastRuntimeStateAt,
			"reportedAtMs":      state.OccurredAt.UnixMilli(),
		},
		},
		"publish device presence changed",
	)
}

func networkBroadcastOnlineDevices(items []model.NetworkDevice) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if !networkMemberOnline(item) {
			continue
		}
		out = append(out, map[string]any{
			"deviceId": item.DeviceID,
			"activePath": item.ActivePath,
			"relayTransport": item.RelayTransport,
			"peerNodeId": item.PeerNodeID,
		})
	}
	return out
}

func mqttBrokerURLForClient(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse mqtt broker url: %w", err)
	}
	switch u.Scheme {
	case "mqtt", "tcp":
		u.Scheme = "tcp"
	case "mqtts", "ssl", "tls":
		u.Scheme = "ssl"
	default:
		return "", fmt.Errorf("unsupported mqtt broker scheme %q", u.Scheme)
	}
	return u.String(), nil
}
