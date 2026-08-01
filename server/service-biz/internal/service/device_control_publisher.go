package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type DeviceControlEnvelope struct {
	Type      string         `json:"type"`
	MessageID string         `json:"messageId"`
	Payload   map[string]any `json:"payload"`
}

type DeviceControlPublisher interface {
	PublishDeviceControl(ctx context.Context, deviceID string, event DeviceControlEnvelope) error
}

type MqttDeviceControlPublisher struct {
	config mqttkit.Config
}

func NewDeviceControlPublisher(cfg mqttkit.Config) DeviceControlPublisher {
	if !cfg.Enabled || strings.TrimSpace(cfg.BrokerURL) == "" {
		return nil
	}
	return &MqttDeviceControlPublisher{config: cfg}
}

func (p *MqttDeviceControlPublisher) PublishDeviceControl(
	ctx context.Context,
	deviceID string,
	event DeviceControlEnvelope,
) error {
	if p == nil {
		return nil
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || strings.TrimSpace(event.Type) == "" {
		return ErrInvalidArgument
	}
	credential := mqttkit.CredentialForServer(p.config, time.Now())
	if credential == nil {
		return ErrNotImplemented
	}
	brokerURL, err := mqttBrokerURLForClient(credential.BrokerURL)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode device control event: %w", err)
	}
	topic := targetDeviceDownstreamTopic(p.config, deviceID)
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if err := publishClientMessageOnce(ctx, brokerURL, credential, topic, payload, attempt); err != nil {
			lastErr = err
			if ctx.Err() != nil || attempt == 2 {
				break
			}
			if err := mqttRetryDelay(ctx, 200*time.Millisecond); err != nil {
				lastErr = err
				break
			}
			continue
		}
		return nil
	}
	return lastErr
}
