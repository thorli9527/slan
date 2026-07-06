package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func (s ClientMessageService) SendClientMessage(ctx context.Context, input SendClientMessageInput) (SendClientMessageView, error) {
	networkID := normalizeNetworkID(input.NetworkID)
	fromDeviceID := normalizeDeviceID(input.FromDeviceID)
	targetDeviceID := normalizeDeviceID(input.TargetDeviceID)
	body := strings.TrimSpace(input.Body)
	if networkID == "" || fromDeviceID == "" || targetDeviceID == "" || body == "" {
		return SendClientMessageView{}, ErrInvalidArgument
	}

	network, err := requireDeviceNetwork(ctx, s.Networks, networkID)
	if err != nil {
		return SendClientMessageView{}, err
	}
	if err := s.ensureActiveMembers(ctx, network.NetworkID, fromDeviceID, targetDeviceID); err != nil {
		return SendClientMessageView{}, err
	}

	now := currentTimeMillis(s.Now)
	messageID := fmt.Sprintf("clientmsg%d%s", now, targetDeviceID)
	payload := map[string]any{
		"type":      "client_message",
		"messageId": messageID,
		"payload": map[string]any{
			"messageId":      messageID,
			"networkId":      network.NetworkID,
			"fromDeviceId":   fromDeviceID,
			"targetDeviceId": targetDeviceID,
			"body":           body,
			"metadata":       normalizedClientMessageMetadata(input.Metadata),
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return SendClientMessageView{}, fmt.Errorf("encode client message payload: %w", err)
	}
	log.Printf(
		"client message send begin networkId=%s fromDeviceId=%s targetDeviceId=%s bodyBytes=%d",
		network.NetworkID,
		fromDeviceID,
		targetDeviceID,
		len(body),
	)

	topic, err := s.publishToNetworkBroadcast(ctx, network.NetworkID, raw)
	if err != nil {
		log.Printf(
			"client message send error networkId=%s fromDeviceId=%s targetDeviceId=%s error=%v",
			network.NetworkID,
			fromDeviceID,
			targetDeviceID,
			err,
		)
		return SendClientMessageView{}, err
	}
	log.Printf(
		"client message send ok networkId=%s fromDeviceId=%s targetDeviceId=%s topic=%s",
		network.NetworkID,
		fromDeviceID,
		targetDeviceID,
		topic,
	)
	return SendClientMessageView{
		MessageID: messageID,
		Transport: "mqtt",
		QOS:       1,
		Topic:     topic,
	}, nil
}

func (s ClientMessageService) ensureActiveMembers(ctx context.Context, networkID, fromDeviceID, targetDeviceID string) error {
	items, err := s.Networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return err
	}
	var fromMember, targetMember *model.NetworkDevice
	for index := range items {
		item := items[index]
		if item.DeviceID == fromDeviceID {
			fromMember = &item
		}
		if item.DeviceID == targetDeviceID {
			targetMember = &item
		}
	}
	if !clientMessageMemberActive(fromMember) || !clientMessageMemberActive(targetMember) {
		return ErrNotFound
	}
	return nil
}

func clientMessageMemberActive(item *model.NetworkDevice) bool {
	return networkMemberActivePtr(item)
}

func normalizedClientMessageMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	return metadata
}

func (s ClientMessageService) publishToNetworkBroadcast(ctx context.Context, networkID string, payload []byte) (string, error) {
	cfg := s.MQTT
	credential := mqttkit.CredentialForServer(cfg, time.UnixMilli(currentTimeMillis(s.Now)))
	if credential == nil {
		return "", ErrNotImplemented
	}
	brokerURL, err := mqttBrokerURLForClient(credential.BrokerURL)
	if err != nil {
		return "", err
	}
	topic := fmt.Sprintf("%s/networks/%s/broadcast", mqttTopicRoot(cfg), strings.TrimSpace(networkID))
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if err := publishClientMessageOnce(ctx, brokerURL, credential, topic, payload, attempt); err != nil {
			lastErr = err
			if ctx.Err() != nil || attempt >= 2 {
				break
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		return topic, nil
	}
	return "", lastErr
}

func publishClientMessageOnce(
	ctx context.Context,
	brokerURL string,
	credential *mqttkit.Credential,
	topic string,
	payload []byte,
	attempt int,
) error {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(serverMQTTClientID(credential.ClientID, "client-message", attempt)).
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
		return fmt.Errorf("publish client message timeout")
	}
	if err := publishToken.Error(); err != nil {
		return fmt.Errorf("publish client message: %w", err)
	}
	return nil
}

func currentTimeMillis(now func() int64) int64 {
	if now != nil {
		return now()
	}
	return time.Now().UnixMilli()
}
