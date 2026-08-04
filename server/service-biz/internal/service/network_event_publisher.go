package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

type NetworkEventPublisher interface {
	PublishNetworkEvent(ctx context.Context, event NetworkEventEnvelope) error
}

type MqttNetworkEventPublisher struct {
	config     mqttkit.Config
	deliveries repository.NetworkEventDeliveryStore
}

const (
	networkJoinDeliveryRetryInterval = time.Minute
	networkJoinDeliveryTTL           = 24 * time.Hour
)

func NewMqttNetworkEventPublisher(cfg mqttkit.Config, deliveries ...repository.NetworkEventDeliveryStore) *MqttNetworkEventPublisher {
	if !cfg.Enabled || strings.TrimSpace(cfg.BrokerURL) == "" {
		return nil
	}
	publisher := &MqttNetworkEventPublisher{config: cfg}
	if len(deliveries) > 0 {
		publisher.deliveries = deliveries[0]
	}
	return publisher
}

func (p *MqttNetworkEventPublisher) PublishNetworkEvent(
	ctx context.Context,
	event NetworkEventEnvelope,
) error {
	if p == nil {
		return nil
	}
	networkID := strings.TrimSpace(event.NetworkID)
	if networkID == "" {
		return ErrInvalidArgument
	}
	credential := mqttkit.CredentialForServer(
		p.config,
		time.UnixMilli(int64(event.OccurredAt)),
	)
	if credential == nil {
		return ErrNotImplemented
	}
	brokerURL, err := mqttBrokerURLForClient(credential.BrokerURL)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode network event: %w", err)
	}
	if err := p.trackMembershipEvent(ctx, event, payload); err != nil {
		return fmt.Errorf("track network membership event: %w", err)
	}
	topic := fmt.Sprintf("%s/networks/%s/broadcast", mqttTopicRoot(p.config), networkID)
	return p.publishWithRetry(ctx, brokerURL, credential, topic, payload)
}

func (p *MqttNetworkEventPublisher) trackMembershipEvent(ctx context.Context, event NetworkEventEnvelope, payload []byte) error {
	if p.deliveries == nil || event.EventType != NetworkEventMemberAdded {
		return nil
	}
	var added NetworkEventMemberPayload
	if raw, err := json.Marshal(event.Payload); err != nil {
		return err
	} else if err := json.Unmarshal(raw, &added); err != nil {
		return err
	}
	targetDeviceID := strings.TrimSpace(added.Member.DeviceID)
	if targetDeviceID == "" {
		return ErrInvalidArgument
	}
	now := time.UnixMilli(event.OccurredAt).Unix()
	if _, exists, err := p.deliveries.GetNetworkEventDelivery(ctx, event.EventID, targetDeviceID); err != nil {
		return err
	} else if exists {
		return nil
	}
	return p.deliveries.SaveNetworkEventDelivery(ctx, model.NetworkEventDelivery{
		EventID: event.EventID, TargetDeviceID: targetDeviceID, NetworkID: event.NetworkID,
		EventType: string(event.EventType), ConfigVersion: int64(event.Version), Payload: string(payload),
		Status: "pending", Attempts: 1,
		NextRetryAt: now + int64(networkJoinDeliveryRetryInterval.Seconds()),
		ExpiresAt:   now + int64(networkJoinDeliveryTTL.Seconds()),
		CreatedAt:   now, UpdatedAt: now,
	})
}

func (p *MqttNetworkEventPublisher) publishWithRetry(
	ctx context.Context,
	brokerURL string,
	credential *mqttkit.Credential,
	topic string,
	payload []byte,
) error {
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if err := p.publishOnce(ctx, brokerURL, credential, topic, payload, attempt); err != nil {
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

func (p *MqttNetworkEventPublisher) publishOnce(
	ctx context.Context,
	brokerURL string,
	credential *mqttkit.Credential,
	topic string,
	payload []byte,
	attempt int,
) error {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(serverMQTTClientID(credential.ClientID, "network-event", attempt)).
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
		return fmt.Errorf("publish network event timeout")
	}
	if err := publishToken.Error(); err != nil {
		return fmt.Errorf("publish network event: %w", err)
	}
	return nil
}
