package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type NetworkEventPublisher interface {
	PublishNetworkEvent(ctx context.Context, event NetworkEventEnvelope) error
}

type MqttNetworkEventPublisher struct {
	config mqttkit.Config
}

func NewMqttNetworkEventPublisher(cfg mqttkit.Config) *MqttNetworkEventPublisher {
	if !cfg.Enabled || strings.TrimSpace(cfg.BrokerURL) == "" {
		return nil
	}
	return &MqttNetworkEventPublisher{config: cfg}
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
	topic := fmt.Sprintf("%s/networks/%s/broadcast", mqttTopicRoot(p.config), networkID)
	return p.publishWithRetry(ctx, brokerURL, credential, topic, payload)
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
