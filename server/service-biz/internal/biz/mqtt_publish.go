package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

func publishControlMQTT(ctx context.Context, cfg MQTTConfig, deviceID, msgType string, payload any) error {
	messageID := mqttMessageID("msg")
	return publishControlMQTTWithMessageID(ctx, cfg, deviceID, msgType, messageID, payload)
}

func publishControlMQTTWithMessageID(ctx context.Context, cfg MQTTConfig, deviceID, msgType, messageID string, payload any) error {
	credential := serverMQTTCredential(cfg, time.Now())
	if credential == nil {
		return errUnavailable
	}
	topic := mqttControlDownTopic(cfg, deviceID)
	return mqttPublishJSON(ctx, cfg, credential.ClientID+"-"+messageID, credential.Username, credential.Password, topic, newControlEnvelope(cfg, msgType, messageID, payload), mqttQoSExactlyOnce)
}

func mqttMessageID(prefix string) string {
	token, err := secureTokenHex(12)
	if err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + token
}

func newControlEnvelope(cfg MQTTConfig, msgType, messageID string, payload any) controlEnvelope {
	now := timeNow().Unix()
	ttl := cfg.ControlMessageTTLSeconds
	if ttl <= 0 {
		ttl = defaultControlMessageTTLSeconds
	}
	return controlEnvelope{
		Type:          msgType,
		MessageID:     strings.TrimSpace(messageID),
		SchemaVersion: mqttControlSchemaVersion,
		CreatedAt:     now,
		ExpiresAt:     now + int64(ttl),
		Payload:       payload,
	}
}

func mqttPublishJSON(ctx context.Context, cfg MQTTConfig, clientID, username, password, topic string, payload any, qos byte) error {
	if !cfg.Enabled {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return mqttPublish(ctx, cfg, clientID, username, password, topic, body, qos)
}

func mqttPublish(ctx context.Context, cfg MQTTConfig, clientID, username, password, topic string, payload []byte, qos byte) error {
	address, err := mqttBrokerAddress(cfg.BrokerURL)
	if err != nil {
		return err
	}
	timeout := time.Duration(cfg.PublishTimeoutMilliseconds) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	refreshMQTTDeadline(conn, timeout)
	if _, err := conn.Write(mqttConnectPacket(clientID, username, password)); err != nil {
		return err
	}
	refreshMQTTDeadline(conn, timeout)
	if err := mqttReadConnAck(conn); err != nil {
		return err
	}
	packet, err := mqttPublishPacket(topic, payload, qos, 1)
	if err != nil {
		return err
	}
	refreshMQTTDeadline(conn, timeout)
	if _, err := conn.Write(packet); err != nil {
		return err
	}
	if qos == mqttQoSExactlyOnce {
		if err := mqttCompleteQoS2(conn, 1, timeout); err != nil {
			return err
		}
	}
	_, _ = conn.Write([]byte{0xe0, 0x00})
	return nil
}
