package mqttauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
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
	username := joinUsername(cfg, deviceID, expiresAt)
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
	username := joinSystemUsername(cfg, serverSubscriberID, expiresAt)
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
	return validateDeviceAt(cfg, clientID, username, givenPassword, time.Now())
}

func validateDeviceAt(cfg configs.MQTTConfig, clientID, username, givenPassword string, now time.Time) (string, bool) {
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
	expiresAt, ok := parseUsernameExpiry(cfg, username, deviceID)
	if !ok || expiresAt < now.Unix() {
		return "", false
	}
	want := password(cfg, clientID, username, deviceID)
	if hmac.Equal([]byte(want), []byte(givenPassword)) {
		return deviceID, true
	}
	return "", false
}

func ValidateCredential(cfg configs.MQTTConfig, clientID, username, givenPassword string) (dto.MQTTAuthCheckResponse, bool) {
	return validateCredentialAt(cfg, clientID, username, givenPassword, time.Now())
}

func validateCredentialAt(cfg configs.MQTTConfig, clientID, username, givenPassword string, now time.Time) (dto.MQTTAuthCheckResponse, bool) {
	if deviceID, ok := validateDeviceAt(cfg, clientID, username, givenPassword, now); ok {
		return dto.MQTTAuthCheckResponse{Allow: true, DeviceID: deviceID, Principal: "device"}, true
	}
	if validateServerSubscriberAt(cfg, clientID, username, givenPassword, now) {
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
		return false
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
	return validateServerSubscriberAt(cfg, clientID, username, givenPassword, time.Now())
}

func validateServerSubscriberAt(cfg configs.MQTTConfig, clientID, username, givenPassword string, now time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	if clientID != joinClientID(cfg, "server") {
		return false
	}
	expiresAt, ok := parseSystemUsernameExpiry(cfg, username, serverSubscriberID)
	if !ok || expiresAt < now.Unix() {
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

func joinUsername(cfg configs.MQTTConfig, id string, expiresAt int64) string {
	return fmt.Sprintf("%s/%s/%d", strings.TrimSpace(cfg.UsernamePrefix), strings.TrimSpace(id), expiresAt)
}

func joinSystemUsername(cfg configs.MQTTConfig, name string, expiresAt int64) string {
	return fmt.Sprintf("%s/system/%s/%d", strings.TrimSpace(cfg.UsernamePrefix), strings.TrimSpace(name), expiresAt)
}

func parseUsernameExpiry(cfg configs.MQTTConfig, username, id string) (int64, bool) {
	prefix := fmt.Sprintf("%s/%s/", strings.TrimSpace(cfg.UsernamePrefix), strings.TrimSpace(id))
	return parseExpirySuffix(username, prefix)
}

func parseSystemUsernameExpiry(cfg configs.MQTTConfig, username, name string) (int64, bool) {
	prefix := fmt.Sprintf("%s/system/%s/", strings.TrimSpace(cfg.UsernamePrefix), strings.TrimSpace(name))
	return parseExpirySuffix(username, prefix)
}

func parseExpirySuffix(username, prefix string) (int64, bool) {
	if !strings.HasPrefix(username, prefix) {
		return 0, false
	}
	expiresAt, err := strconv.ParseInt(strings.TrimPrefix(username, prefix), 10, 64)
	if err != nil || expiresAt <= 0 {
		return 0, false
	}
	return expiresAt, true
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
