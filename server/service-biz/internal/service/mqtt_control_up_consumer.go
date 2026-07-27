package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func (s MQTTWebhookService) StartControlUpConsumer(ctx context.Context) error {
	if !s.Config.Enabled || strings.TrimSpace(s.Config.BrokerURL) == "" {
		log.Printf("mqtt control/up consumer disabled enabled=%t broker=%q", s.Config.Enabled, s.Config.BrokerURL)
		return nil
	}
	credential := mqttkit.CredentialForServer(s.Config, currentTime(s.Now))
	if credential == nil {
		return nil
	}
	brokerURL, err := mqttBrokerURLForClient(consumerBrokerURL(s.Config, credential.BrokerURL))
	if err != nil {
		return err
	}
	topicRoot := mqttTopicRoot(s.Config)
	topics := map[string]byte{
		fmt.Sprintf("%s/devices/+/control/up", topicRoot):    1,
		fmt.Sprintf("%s/devices/+/heartbeat", topicRoot):     0,
		fmt.Sprintf("%s/devices/+/runtime-state", topicRoot): 0,
	}
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(credential.ClientID + "-control-up").
		SetConnectTimeout(5 * time.Second).
		SetWriteTimeout(5 * time.Second).
		SetOrderMatters(false).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second)
	opts.SetCredentialsProvider(func() (string, string) {
		fresh := mqttkit.CredentialForServer(s.Config, currentTime(s.Now))
		if fresh == nil {
			return credential.Username, credential.Password
		}
		return fresh.Username, fresh.Password
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		log.Printf("mqtt control/up consumer connection lost: %v", err)
	})
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		token := client.SubscribeMultiple(topics, func(_ mqtt.Client, message mqtt.Message) {
			msgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.HandleUpstreamMessage(msgCtx, message.Topic(), message.Payload()); err != nil {
				log.Printf("mqtt upstream consume failed topic=%s err=%v payload=%s", message.Topic(), err, string(message.Payload()))
			}
		})
		if ok := token.WaitTimeout(5 * time.Second); !ok {
			log.Printf("mqtt upstream consumer subscribe timeout topics=%v", topics)
			return
		}
		if err := token.Error(); err != nil {
			log.Printf("mqtt upstream consumer subscribe failed topics=%v err=%v", topics, err)
			return
		}
		log.Printf("mqtt upstream consumer subscribed topics=%v", topics)
	})
	client := mqtt.NewClient(opts)
	connectToken := client.Connect()
	if ok := connectToken.WaitTimeout(6 * time.Second); !ok {
		return fmt.Errorf("connect mqtt broker timeout")
	}
	if err := connectToken.Error(); err != nil {
		return fmt.Errorf("connect mqtt broker: %w", err)
	}
	go func() {
		<-ctx.Done()
		client.Disconnect(250)
	}()
	return nil
}

func (s MQTTWebhookService) HandleUpstreamMessage(ctx context.Context, topic string, payload []byte) error {
	switch {
	case strings.HasSuffix(topic, "/control/up"):
		return s.HandleControlUpMessage(ctx, topic, payload)
	case strings.HasSuffix(topic, "/heartbeat"):
		return s.handlePresenceMessage(ctx, topic, payload, true)
	case strings.HasSuffix(topic, "/runtime-state"):
		return s.handlePresenceMessage(ctx, topic, payload, false)
	default:
		return fmt.Errorf("unsupported mqtt upstream topic: %s", topic)
	}
}

func (s MQTTWebhookService) handlePresenceMessage(ctx context.Context, topic string, payload []byte, heartbeat bool) error {
	var values map[string]any
	if err := json.Unmarshal(payload, &values); err != nil {
		return fmt.Errorf("decode mqtt presence payload: %w", err)
	}
	deviceID := deviceIDFromUpstreamTopic(topic)
	payloadDeviceID := stringMapValue(values, "deviceId")
	if deviceID == "" || payloadDeviceID == "" || payloadDeviceID != deviceID {
		return fmt.Errorf("mqtt presence device mismatch topic=%s payloadDeviceId=%s", deviceID, payloadDeviceID)
	}
	networkID := stringMapValue(values, "activeNetworkId")
	if networkID == "" {
		return fmt.Errorf("mqtt presence active network missing deviceId=%s", deviceID)
	}
	items, err := s.Networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return err
	}
	now := currentTime(s.Now)
	for _, item := range items {
		if item.DeviceID != deviceID || !networkMemberActive(item) {
			continue
		}
		wasOnline := networkMemberOnlineAt(item, now)
		updated := item
		updated.LastSeenAt = now.Unix()
		if heartbeat {
			updated.LastHeartbeatAt = now.Unix()
		} else {
			updated.LastRuntimeStateAt = now.Unix()
		}
		updated.PresenceStatus = model.DevicePresenceStatusActive
		updated.UpdatedAt = now.Unix()
		if err := s.Networks.SaveNetworkDevice(ctx, updated); err != nil {
			return err
		}
		if !wasOnline {
			return publishDevicePresenceChanged(ctx, s.EventPublisher, now, networkID, deviceID, updated)
		}
		return nil
	}
	return ErrNotFound
}

func consumerBrokerURL(cfg mqttkit.Config, fallback string) string {
	if value := strings.TrimSpace(os.Getenv("SLAN_MQTT_BROKER_URL")); value != "" {
		return value
	}
	if value := strings.TrimSpace(cfg.BrokerURL); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func (s MQTTWebhookService) HandleControlUpMessage(ctx context.Context, topic string, payload []byte) error {
	var envelope MQTTControlUpEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("decode control/up envelope: %w", err)
	}
	messageType := strings.TrimSpace(envelope.Type)
	switch messageType {
	case "endpoint_report":
		input, err := decodeControlUpEndpointReport(topic, envelope)
		if err != nil {
			return err
		}
		_, err = s.ReportEndpoint(ctx, input)
		return err
	case "path_health_report":
		input, err := decodeControlUpPathHealthReport(topic, envelope)
		if err != nil {
			return err
		}
		return s.ReportPathHealth(ctx, input)
	default:
		log.Printf("mqtt control/up ignore topic=%s type=%s", topic, messageType)
		return nil
	}
}

func decodeControlUpEndpointReport(topic string, envelope MQTTControlUpEnvelope) (MQTTEndpointReportInput, error) {
	values, err := decodeControlUpPayload(envelope)
	if err != nil {
		return MQTTEndpointReportInput{}, err
	}
	networkID := firstNonEmpty(stringMapValue(values, "networkId"), strings.TrimSpace(envelope.NetworkID))
	deviceID := firstNonEmpty(stringMapValue(values, "deviceId"), deviceIDFromControlUpTopic(topic))
	nodeID := stringMapValue(values, "nodeId")
	if deviceID == "" && strings.HasPrefix(nodeID, "node-") {
		deviceID = strings.TrimPrefix(nodeID, "node-")
	}
	endpoints := make([]DeviceEndpointView, 0)
	if items, ok := values["endpoints"].([]any); ok {
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			endpoints = append(endpoints, DeviceEndpointView{
				Type:      firstNonEmpty(stringMapValue(entry, "type"), stringMapValue(entry, "kind")),
				Address:   firstNonEmpty(stringMapValue(entry, "address"), stringMapValue(entry, "endpoint")),
				UpdatedAt: int64MapValue(entry, "updatedAt"),
			})
		}
	}
	return MQTTEndpointReportInput{
		NetworkID: networkID,
		DeviceID:  deviceID,
		NodeID:    nodeID,
		NATType:   stringMapValue(values, "natType"),
		Endpoints: endpoints,
	}, nil
}

func decodeControlUpPathHealthReport(topic string, envelope MQTTControlUpEnvelope) (MQTTPathHealthReportInput, error) {
	values, err := decodeControlUpPayload(envelope)
	if err != nil {
		return MQTTPathHealthReportInput{}, err
	}
	return MQTTPathHealthReportInput{
		NetworkID:         firstNonEmpty(stringMapValue(values, "networkId"), strings.TrimSpace(envelope.NetworkID)),
		DeviceID:          firstNonEmpty(stringMapValue(values, "deviceId"), deviceIDFromControlUpTopic(topic)),
		PeerNodeID:        stringMapValue(values, "peerNodeId"),
		PathType:          stringMapValue(values, "pathType"),
		ActivePath:        stringMapValue(values, "activePath"),
		RelayTransport:    stringMapValue(values, "relayTransport"),
		Endpoint:          stringMapValue(values, "endpoint"),
		DerpNodeID:        stringMapValue(values, "derpNodeId"),
		ObservedRttMs:     int64MapValue(values, "observedRttMs"),
		PacketLossPpm:     int64MapValue(values, "packetLossPpm"),
		PathScore:         int64MapValue(values, "pathScore"),
		RelayMtu:          int(int64MapValue(values, "relayMtu")),
		MaxFramePayload:   int(int64MapValue(values, "maxFramePayload")),
		TicketExpiresAt:   stringMapValue(values, "ticketExpiresAt"),
		TicketExpiresInMs: int64MapValue(values, "ticketExpiresInMs"),
		TicketRenewDue:    boolMapValue(values, "ticketRenewDue"),
		PathDowngrades:    int64MapValue(values, "pathDowngrades"),
		PathUpgrades:      int64MapValue(values, "pathUpgrades"),
		LastPathChange:    stringMapValue(values, "lastPathChange"),
		SampledAtMs:       int64MapValue(values, "sampledAtMs"),
	}, nil
}

func decodeControlUpPayload(envelope MQTTControlUpEnvelope) (map[string]any, error) {
	if len(envelope.Payload) == 0 {
		return map[string]any{}, nil
	}
	var values map[string]any
	if err := json.Unmarshal(envelope.Payload, &values); err != nil {
		return nil, fmt.Errorf("decode control/up payload: %w", err)
	}
	return values, nil
}

func deviceIDFromControlUpTopic(topic string) string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(topic), "/"), "/")
	if len(parts) != 5 || parts[1] != "devices" || parts[3] != "control" || parts[4] != "up" {
		return ""
	}
	return parts[2]
}

func deviceIDFromUpstreamTopic(topic string) string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(topic), "/"), "/")
	if len(parts) < 4 || parts[1] != "devices" {
		return ""
	}
	return parts[2]
}

func stringMapValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func int64MapValue(values map[string]any, key string) int64 {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		var parsed int64
		_, _ = fmt.Sscan(strings.TrimSpace(typed), &parsed)
		return parsed
	default:
		return 0
	}
}

func boolMapValue(values map[string]any, key string) bool {
	value, ok := values[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		text := strings.ToLower(strings.TrimSpace(typed))
		return text == "true" || text == "1" || text == "yes"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}
