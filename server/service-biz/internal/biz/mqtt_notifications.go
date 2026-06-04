package biz

import (
	"context"
	"log"
	"strings"
	"time"
)

func (s *Server) notifyNetworkConfigChanged(networkID, reason, resourceType, action, resourceID, deviceID string) {
	if !s.mqtt.Enabled || strings.TrimSpace(networkID) == "" {
		return
	}
	now := timeNow().Unix()
	version := s.services.MQTT.NextNetworkConfigVersion(networkID, now)
	payload := networkChangePayload{
		NetworkID:     networkID,
		ConfigVersion: version,
		Reason:        reason,
		ChangedAt:     now,
		ResourceType:  resourceType,
		Action:        action,
		ResourceID:    resourceID,
		DeviceID:      deviceID,
	}
	deviceIDs := s.services.MQTT.ListActiveNetworkDeviceIDs(networkID)
	if deviceID != "" && !containsString(deviceIDs, deviceID) {
		deviceIDs = append(deviceIDs, deviceID)
	}
	go s.publishNetworkConfigChanged(deviceIDs, payload)
}

func (s *Server) notifyDeviceNetworkConfigsChanged(configs []NetworkConfig, reason, resourceType, action, deviceID string) {
	seen := make(map[string]struct{}, len(configs))
	for _, config := range configs {
		networkID := strings.TrimSpace(config.NetworkID)
		if networkID == "" {
			continue
		}
		if _, ok := seen[networkID]; ok {
			continue
		}
		seen[networkID] = struct{}{}
		s.notifyNetworkConfigChanged(networkID, reason, resourceType, action, deviceID, deviceID)
		s.notifyNetworkMemberState(networkID, deviceID, "enabled", reason)
	}
}
func (s *Server) notifyNetworkMemberState(networkID, deviceID, state, reason string) {
	if !s.mqtt.Enabled || strings.TrimSpace(networkID) == "" || strings.TrimSpace(deviceID) == "" {
		return
	}
	effectiveState := state
	if state == "expired" {
		effectiveState = "disabled"
	}
	now := timeNow().Unix()
	payload := networkChangePayload{
		NetworkID:      networkID,
		Reason:         reason,
		ChangedAt:      now,
		ResourceType:   "network_member",
		Action:         effectiveState,
		DeviceID:       deviceID,
		EffectiveState: effectiveState,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
		defer cancel()
		topic := mqttNetworkBroadcastTopic(s.mqtt, networkID)
		credential := serverMQTTCredential(s.mqtt, timeNow())
		if credential == nil {
			return
		}
		messageID := mqttMessageID("msg")
		clientID := credential.ClientID + "-member-" + messageID
		err := mqttPublishJSON(ctx, s.mqtt, clientID, credential.Username, credential.Password, topic, newControlEnvelope(s.mqtt, "network_member_state_changed", messageID, payload), mqttQoSExactlyOnce)
		if err != nil {
			log.Printf("mqtt publish network_member_state_changed failed device=%s network=%s: %v", deviceID, networkID, err)
		}
	}()
}

func (s *Server) notifyDeviceNetworkPresence(networkID, deviceID string, online bool, changedAt int64) {
	if !s.mqtt.Enabled || strings.TrimSpace(networkID) == "" || strings.TrimSpace(deviceID) == "" {
		return
	}
	messageType := "device_network_disabled"
	if online {
		messageType = "device_network_enabled"
	}
	payload := map[string]any{
		"networkId": networkID,
		"deviceId":  deviceID,
		"online":    online,
		"changedAt": changedAt,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
		defer cancel()
		topic := mqttNetworkBroadcastTopic(s.mqtt, networkID)
		credential := serverMQTTCredential(s.mqtt, timeNow())
		if credential == nil {
			return
		}
		messageID := mqttMessageID("msg")
		clientID := credential.ClientID + "-presence-" + messageID
		err := mqttPublishJSON(ctx, s.mqtt, clientID, credential.Username, credential.Password, topic, newControlEnvelope(s.mqtt, messageType, messageID, payload), mqttQoSExactlyOnce)
		if err != nil {
			log.Printf("mqtt publish %s failed device=%s network=%s: %v", messageType, deviceID, networkID, err)
		}
	}()
}
func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func securityRuleReason(direction, action string) string {
	prefix := "security_rule_ingress"
	if strings.EqualFold(direction, "egress") {
		prefix = "security_rule_egress"
	}
	return prefix + "_" + action
}
