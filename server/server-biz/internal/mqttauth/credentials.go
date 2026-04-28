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
	deviceID, expiresAt, ok := parseDeviceUsername(cfg, username)
	if !ok || expiresAt < now.Unix() {
		return "", false
	}
	baseClientID := joinClientID(cfg, deviceID)
	if clientID != baseClientID && !strings.HasPrefix(clientID, baseClientID+"-") {
		return "", false
	}
	if hmac.Equal([]byte(password(cfg, clientID, username, deviceID)), []byte(givenPassword)) ||
		hmac.Equal([]byte(password(cfg, baseClientID, username, deviceID)), []byte(givenPassword)) {
		return deviceID, true
	}
	return "", false
}

func ValidateCredential(cfg configs.MQTTConfig, clientID, username, givenPassword string) (dto.MQTTCredentialAuthResult, bool) {
	return validateCredentialAt(cfg, clientID, username, givenPassword, time.Now())
}

func validateCredentialAt(cfg configs.MQTTConfig, clientID, username, givenPassword string, now time.Time) (dto.MQTTCredentialAuthResult, bool) {
	if deviceID, ok := validateDeviceAt(cfg, clientID, username, givenPassword, now); ok {
		return dto.MQTTCredentialAuthResult{Allow: true, DeviceID: deviceID, Principal: "device"}, true
	}
	if validateServerSubscriberAt(cfg, clientID, username, givenPassword, now) {
		return dto.MQTTCredentialAuthResult{Allow: true, Principal: "server"}, true
	}
	return dto.MQTTCredentialAuthResult{Allow: false}, false
}

func DeviceTopicPrefix(cfg configs.MQTTConfig, deviceID string) string {
	return trimTopic(cfg.TopicPrefix) + "/" + strings.TrimSpace(deviceID)
}

func NetworkStateTopicFilter(cfg configs.MQTTConfig) string {
	return trimTopic(cfg.TopicPrefix) + "/+/networks/+/state"
}

func ControlUpTopicFilter(cfg configs.MQTTConfig) string {
	return trimTopic(cfg.TopicPrefix) + "/+/control/up"
}

func ControlUpTopic(cfg configs.MQTTConfig, deviceID string) string {
	return DeviceTopicPrefix(cfg, deviceID) + "/control/up"
}

func ControlDownTopic(cfg configs.MQTTConfig, deviceID string) string {
	return DeviceTopicPrefix(cfg, deviceID) + "/control/down"
}

func AllowTopicAccess(cfg configs.MQTTConfig, principal, deviceID, topic string, subscribe bool) bool {
	topic = trimTopic(topic)
	if topic == "" {
		return false
	}
	if principal == "server" {
		if subscribe {
			return topic == NetworkStateTopicFilter(cfg) || topic == ControlUpTopicFilter(cfg)
		}
		return isServerControlDownTopic(cfg, topic)
	}
	if principal != "device" || strings.TrimSpace(deviceID) == "" {
		return false
	}
	devicePrefix := DeviceTopicPrefix(cfg, deviceID)
	if subscribe && topic == devicePrefix+"/#" {
		return true
	}
	if subscribe {
		return topic == ControlDownTopic(cfg, deviceID)
	}
	return topic == ControlUpTopic(cfg, deviceID) || isDeviceNetworkStateTopic(devicePrefix, topic)
}

func isDeviceNetworkStateTopic(devicePrefix, topic string) bool {
	suffix := strings.TrimPrefix(topic, devicePrefix+"/")
	if suffix == topic || suffix == "" {
		return false
	}
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] == "networks" && parts[1] != "" && parts[2] == "state"
}

func isServerControlDownTopic(cfg configs.MQTTConfig, topic string) bool {
	prefix := trimTopic(cfg.TopicPrefix)
	suffix := strings.TrimPrefix(topic, prefix+"/")
	if suffix == topic || suffix == "" {
		return false
	}
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] == "control" && parts[2] == "down"
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

func parseDeviceUsername(cfg configs.MQTTConfig, username string) (string, int64, bool) {
	prefix := strings.TrimSpace(cfg.UsernamePrefix) + "/"
	rest := strings.TrimPrefix(username, prefix)
	if rest == username {
		return "", 0, false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return "", 0, false
	}
	expiresAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || expiresAt <= 0 {
		return "", 0, false
	}
	return strings.TrimSpace(parts[0]), expiresAt, true
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
