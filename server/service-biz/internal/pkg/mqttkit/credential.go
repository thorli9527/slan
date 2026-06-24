package mqttkit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type Credential struct {
	BrokerURL   string `json:"brokerUrl"`
	ClientID    string `json:"clientId"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	TopicPrefix string `json:"topicPrefix"`
	ExpiresAt   int64  `json:"expiresAt"`
}

func CredentialForDevice(cfg Config, deviceID string, now time.Time) *Credential {
	if !cfg.Enabled || strings.TrimSpace(deviceID) == "" {
		return nil
	}
	expiresAt := now.Add(cfg.CredentialTTL).Unix()
	clientID := fmt.Sprintf("%s-%s", defaultString(cfg.ClientIDPrefix, "slan-device"), strings.TrimSpace(deviceID))
	username := fmt.Sprintf("%s:%s:%d", defaultString(cfg.UsernamePrefix, "device"), strings.TrimSpace(deviceID), expiresAt)
	password := sign(cfg.Secret, clientID, username, deviceID)
	return &Credential{
		BrokerURL:   cfg.BrokerURL,
		ClientID:    clientID,
		Username:    username,
		Password:    password,
		TopicPrefix: deviceTopicPrefix(cfg, deviceID),
		ExpiresAt:   expiresAt,
	}
}

func sign(secret string, parts ...string) string {
	payload := strings.Join(parts, "|") + "|" + secret
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
