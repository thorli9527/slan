package biz

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func deviceMQTTCredential(cfg MQTTConfig, deviceID string, now time.Time) *MQTTCredential {
	deviceID = strings.TrimSpace(deviceID)
	if !cfg.Enabled || deviceID == "" {
		return nil
	}
	expiresAt := now.Add(time.Duration(cfg.CredentialTTLSeconds) * time.Second).Unix()
	clientID := mqttClientID(cfg, deviceID)
	username := mqttUsername(cfg, deviceID, expiresAt)
	return &MQTTCredential{
		BrokerURL:   mqttPublicBrokerURL(cfg),
		ClientID:    clientID,
		Username:    username,
		Password:    mqttPassword(cfg, clientID, username, deviceID),
		TopicPrefix: mqttDeviceTopicPrefix(cfg, deviceID),
		ExpiresAt:   expiresAt,
	}
}

func serverMQTTCredential(cfg MQTTConfig, now time.Time) *MQTTCredential {
	if !cfg.Enabled {
		return nil
	}
	expiresAt := now.Add(time.Duration(cfg.CredentialTTLSeconds) * time.Second).Unix()
	clientID := mqttClientID(cfg, "server")
	username := mqttSystemUsername(cfg, mqttServerID, expiresAt)
	return &MQTTCredential{
		BrokerURL:   cfg.BrokerURL,
		ClientID:    clientID,
		Username:    username,
		Password:    mqttPassword(cfg, clientID, username, mqttServerID),
		TopicPrefix: mqttTopicRoot(cfg),
		ExpiresAt:   expiresAt,
	}
}

func validateMQTTCredential(cfg MQTTConfig, clientID, username, givenPassword string, now time.Time) (mqttAuthResult, bool) {
	if deviceID, ok := validateDeviceMQTTCredential(cfg, clientID, username, givenPassword, now); ok {
		return mqttAuthResult{Principal: "device", DeviceID: deviceID}, true
	}
	if validateServerMQTTCredential(cfg, clientID, username, givenPassword, now) {
		return mqttAuthResult{Principal: "server"}, true
	}
	return mqttAuthResult{}, false
}

func validateDeviceMQTTCredential(cfg MQTTConfig, clientID, username, givenPassword string, now time.Time) (string, bool) {
	if !cfg.Enabled {
		return "", false
	}
	deviceID, expiresAt, ok := parseMQTTUsername(cfg, username)
	if !ok || expiresAt < now.Unix() {
		return "", false
	}
	baseClientID := mqttClientID(cfg, deviceID)
	if clientID != baseClientID && !strings.HasPrefix(clientID, baseClientID+"-") {
		return "", false
	}
	return deviceID, hmac.Equal([]byte(mqttPassword(cfg, clientID, username, deviceID)), []byte(givenPassword)) ||
		hmac.Equal([]byte(mqttPassword(cfg, baseClientID, username, deviceID)), []byte(givenPassword))
}

func validateServerMQTTCredential(cfg MQTTConfig, clientID, username, givenPassword string, now time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	expiresAt, ok := parseMQTTSystemUsername(cfg, username, mqttServerID)
	if !ok || expiresAt < now.Unix() {
		return false
	}
	baseClientID := mqttClientID(cfg, "server")
	if clientID != baseClientID && !strings.HasPrefix(clientID, baseClientID+"-") {
		return false
	}
	return hmac.Equal([]byte(mqttPassword(cfg, clientID, username, mqttServerID)), []byte(givenPassword)) ||
		hmac.Equal([]byte(mqttPassword(cfg, baseClientID, username, mqttServerID)), []byte(givenPassword))
}

func mqttClientID(cfg MQTTConfig, id string) string {
	return sanitizeDNSLabel(cfg.UsernamePrefix) + "-" + strings.ReplaceAll(strings.TrimSpace(id), "/", "-")
}

func mqttUsername(cfg MQTTConfig, id string, expiresAt int64) string {
	return fmt.Sprintf("%s:%s:%d", strings.TrimSpace(cfg.UsernamePrefix), strings.TrimSpace(id), expiresAt)
}

func mqttSystemUsername(cfg MQTTConfig, id string, expiresAt int64) string {
	return fmt.Sprintf("%s:system:%s:%d", strings.TrimSpace(cfg.UsernamePrefix), strings.TrimSpace(id), expiresAt)
}

func parseMQTTUsername(cfg MQTTConfig, username string) (string, int64, bool) {
	parts := strings.Split(strings.TrimSpace(username), ":")
	if len(parts) != 3 || parts[0] != strings.TrimSpace(cfg.UsernamePrefix) {
		return "", 0, false
	}
	expiresAt, err := strconv.ParseInt(parts[2], 10, 64)
	return parts[1], expiresAt, err == nil && parts[1] != ""
}

func parseMQTTSystemUsername(cfg MQTTConfig, username, expectedID string) (int64, bool) {
	parts := strings.Split(strings.TrimSpace(username), ":")
	if len(parts) != 4 || parts[0] != strings.TrimSpace(cfg.UsernamePrefix) || parts[1] != "system" || parts[2] != expectedID {
		return 0, false
	}
	expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
	return expiresAt, err == nil
}

func mqttPassword(cfg MQTTConfig, clientID, username, id string) string {
	mac := hmac.New(sha256.New, []byte(cfg.PasswordSecret))
	mac.Write([]byte(clientID))
	mac.Write([]byte{0})
	mac.Write([]byte(username))
	mac.Write([]byte{0})
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

func mqttPublicBrokerURL(cfg MQTTConfig) string {
	if strings.TrimSpace(cfg.PublicBrokerURL) != "" {
		return strings.TrimSpace(cfg.PublicBrokerURL)
	}
	return strings.TrimSpace(cfg.BrokerURL)
}
