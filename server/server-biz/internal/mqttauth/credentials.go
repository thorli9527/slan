package mqttauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
)

const serverSubscriberID = "server-biz-subscriber"

func DeviceCredential(cfg configs.MQTTConfig, deviceID, _ string, now time.Time) *dto.MQTTCredential {
	if !cfg.Enabled || strings.TrimSpace(deviceID) == "" {
		return nil
	}
	expiresAt := expiresAtUnix(cfg, now)
	clientID := joinClientID(cfg, deviceID)
	username := joinUsername(cfg, deviceID)
	return &dto.MQTTCredential{
		BrokerURL:   publicBrokerURL(cfg),
		ClientID:    clientID,
		Username:    username,
		Password:    password(cfg, clientID, username, deviceID),
		TopicPrefix: trimTopic(cfg.TopicPrefix) + "/" + deviceID,
		ExpiresAt:   expiresAt,
	}
}

func ServerSubscriberCredential(cfg configs.MQTTConfig, now time.Time) *dto.MQTTCredential {
	if !cfg.Enabled {
		return nil
	}
	expiresAt := expiresAtUnix(cfg, now)
	clientID := joinClientID(cfg, "server")
	username := joinSystemUsername(cfg, serverSubscriberID)
	return &dto.MQTTCredential{
		BrokerURL:   brokerURL(cfg),
		ClientID:    clientID,
		Username:    username,
		Password:    password(cfg, clientID, username, serverSubscriberID),
		TopicPrefix: trimTopic(cfg.TopicPrefix),
		ExpiresAt:   expiresAt,
	}
}

func Validate(cfg configs.MQTTConfig, clientID, username, givenPassword string) bool {
	_, ok := ValidateDevice(cfg, clientID, username, givenPassword)
	return ok
}

func ValidateDevice(cfg configs.MQTTConfig, clientID, username, givenPassword string) (string, bool) {
	if !cfg.Enabled {
		return "", false
	}
	prefix := strings.TrimSpace(cfg.UsernamePrefix) + "-"
	if !strings.HasPrefix(clientID, prefix) {
		return "", false
	}
	deviceID := strings.TrimPrefix(clientID, prefix)
	if deviceID == "" {
		return "", false
	}
	if username != joinUsername(cfg, deviceID) {
		return "", false
	}
	want := password(cfg, clientID, username, deviceID)
	if hmac.Equal([]byte(want), []byte(givenPassword)) {
		return deviceID, true
	}
	return "", false
}

func ValidateCredential(cfg configs.MQTTConfig, clientID, username, givenPassword string) (dto.MQTTAuthCheckResponse, bool) {
	if deviceID, ok := ValidateDevice(cfg, clientID, username, givenPassword); ok {
		return dto.MQTTAuthCheckResponse{Allow: true, DeviceID: deviceID, Principal: "device"}, true
	}
	if validateServerSubscriber(cfg, clientID, username, givenPassword) {
		return dto.MQTTAuthCheckResponse{Allow: true, Principal: "server"}, true
	}
	return dto.MQTTAuthCheckResponse{Allow: false}, false
}

func DeviceTopicPrefix(cfg configs.MQTTConfig, deviceID string) string {
	return trimTopic(cfg.TopicPrefix) + "/" + strings.TrimSpace(deviceID)
}

func NetworkStateTopicFilter(cfg configs.MQTTConfig) string {
	return trimTopic(cfg.TopicPrefix) + "/+/networks/+/state"
}

func AllowTopicAccess(cfg configs.MQTTConfig, principal, deviceID, topic string, subscribe bool) bool {
	topic = trimTopic(topic)
	if topic == "" {
		return false
	}
	if principal == "server" {
		if subscribe {
			return topic == NetworkStateTopicFilter(cfg)
		}
		return strings.HasPrefix(topic, trimTopic(cfg.TopicPrefix)+"/")
	}
	if principal != "device" || strings.TrimSpace(deviceID) == "" {
		return false
	}
	devicePrefix := DeviceTopicPrefix(cfg, deviceID)
	if subscribe && topic == devicePrefix+"/#" {
		return true
	}
	return !subscribe && isDeviceNetworkStateTopic(devicePrefix, topic)
}

func isDeviceNetworkStateTopic(devicePrefix, topic string) bool {
	suffix := strings.TrimPrefix(topic, devicePrefix+"/")
	if suffix == topic || suffix == "" {
		return false
	}
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] == "networks" && parts[1] != "" && parts[2] == "state"
}

func validateServerSubscriber(cfg configs.MQTTConfig, clientID, username, givenPassword string) bool {
	if !cfg.Enabled {
		return false
	}
	if clientID != joinClientID(cfg, "server") || username != joinSystemUsername(cfg, serverSubscriberID) {
		return false
	}
	want := password(cfg, clientID, username, serverSubscriberID)
	return hmac.Equal([]byte(want), []byte(givenPassword))
}

func expiresAtUnix(cfg configs.MQTTConfig, now time.Time) int64 {
	ttl := cfg.CredentialTTLSeconds
	if ttl <= 0 {
		ttl = 86400
	}
	return now.Add(time.Duration(ttl) * time.Second).Unix()
}

func password(cfg configs.MQTTConfig, clientID, username, id string) string {
	mac := hmac.New(sha256.New, []byte(cfg.PasswordSecret))
	mac.Write([]byte(clientID))
	mac.Write([]byte{0})
	mac.Write([]byte(username))
	mac.Write([]byte{0})
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

func joinClientID(cfg configs.MQTTConfig, id string) string {
	return strings.TrimSpace(cfg.UsernamePrefix) + "-" + strings.TrimSpace(id)
}

func joinUsername(cfg configs.MQTTConfig, id string) string {
	return strings.TrimSpace(cfg.UsernamePrefix) + "/" + strings.TrimSpace(id)
}

func joinSystemUsername(cfg configs.MQTTConfig, name string) string {
	return fmt.Sprintf("%s/system/%s", strings.TrimSpace(cfg.UsernamePrefix), strings.TrimSpace(name))
}

func publicBrokerURL(cfg configs.MQTTConfig) string {
	if strings.TrimSpace(cfg.PublicBrokerURL) != "" {
		return strings.TrimSpace(cfg.PublicBrokerURL)
	}
	return strings.TrimSpace(cfg.BrokerURL)
}

func brokerURL(cfg configs.MQTTConfig) string {
	return strings.TrimSpace(cfg.BrokerURL)
}

func trimTopic(topic string) string {
	value := strings.Trim(strings.TrimSpace(topic), "/")
	if value == "" {
		return "slan"
	}
	return value
}
