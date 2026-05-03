package impl

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/mqttauth"
)

func (s *dbState) startMQTTNetworkStateSubscriber() {
	if !s.cfg.MQTT.Enabled {
		return
	}
	topicFilter := mqttauth.NetworkStateTopicFilter(s.cfg.MQTT)
	go func() {
		for {
			credential := mqttauth.ServerSubscriberCredential(s.cfg.MQTT, time.Now())
			if credential == nil {
				return
			}
			err := mqttauth.Subscribe(
				context.Background(),
				s.cfg.MQTT,
				credential.ClientID,
				credential.Username,
				credential.Password,
				topicFilter,
				func(topic string, payload []byte) {
					if err := s.applyMQTTNetworkState(topic, payload); err != nil {
						log.Printf("mqtt network state ignored topic=%s err=%v", topic, err)
					}
				},
			)
			if err != nil {
				log.Printf("mqtt network state subscriber disconnected: %v", err)
			}
			time.Sleep(3 * time.Second)
		}
	}()
}

func (s *dbState) applyMQTTNetworkState(topic string, payload []byte) error {
	deviceID, networkID, ok := parseNetworkStateTopic(s.cfg.MQTT.TopicPrefix, topic)
	if !ok {
		return ErrInvalidArgument
	}
	var req dto.DeviceNetworkStateRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}
	if strings.TrimSpace(req.DeviceID) != "" && strings.TrimSpace(req.DeviceID) != deviceID {
		return ErrForbidden
	}
	if strings.TrimSpace(req.NetworkID) != "" && strings.TrimSpace(req.NetworkID) != networkID {
		return ErrForbidden
	}
	return s.upsertTrustedDeviceNetworkState(context.Background(), deviceID, networkID, req)
}

func parseNetworkStateTopic(topicPrefix, topic string) (string, string, bool) {
	prefixParts := splitTopic(topicPrefix)
	parts := splitTopic(topic)
	if len(parts) != len(prefixParts)+4 {
		return "", "", false
	}
	for index, prefixPart := range prefixParts {
		if parts[index] != prefixPart {
			return "", "", false
		}
	}
	deviceID := parts[len(prefixParts)]
	if parts[len(prefixParts)+1] != "networks" {
		return "", "", false
	}
	networkID := parts[len(prefixParts)+2]
	if parts[len(prefixParts)+3] != "state" {
		return "", "", false
	}
	if deviceID == "" || networkID == "" {
		return "", "", false
	}
	return deviceID, networkID, true
}

func splitTopic(topic string) []string {
	trimmed := strings.Trim(strings.TrimSpace(topic), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
