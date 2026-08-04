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

func CredentialForDevice(cfg Config, deviceID, credentialID string, now time.Time, expiresAtLimit int64) *Credential {
	deviceID = strings.TrimSpace(deviceID)
	credentialID = strings.TrimSpace(credentialID)
	if !cfg.Enabled || deviceID == "" || credentialID == "" {
		return nil
	}
	expiresAt := now.Add(cfg.CredentialTTL).Unix()
	if expiresAtLimit > 0 && expiresAt > expiresAtLimit {
		expiresAt = expiresAtLimit
	}
	clientID := fmt.Sprintf("%s-%s", defaultString(cfg.ClientIDPrefix, "slan-device"), deviceID)
	username := fmt.Sprintf("%s:%s:%s:%d", defaultString(cfg.UsernamePrefix, "device"), deviceID, credentialID, expiresAt)
	password := sign(cfg.Secret, clientID, username, deviceID, credentialID)
	return &Credential{
		BrokerURL:   publicBrokerURL(cfg),
		ClientID:    clientID,
		Username:    username,
		Password:    password,
		TopicPrefix: deviceTopicPrefix(cfg, deviceID),
		ExpiresAt:   expiresAt,
	}
}

func CredentialForSystem(cfg Config, id string, now time.Time) *Credential {
	id = strings.TrimSpace(id)
	if !cfg.Enabled || id == "" {
		return nil
	}
	expiresAt := now.Add(cfg.CredentialTTL).Unix()
	clientID := fmt.Sprintf("%s-%s", defaultString(cfg.ClientIDPrefix, "slan-device"), strings.ReplaceAll(id, "/", "-"))
	username := SystemUsername(cfg, id, expiresAt)
	password := sign(cfg.Secret, clientID, username, id)
	return &Credential{
		BrokerURL:   publicBrokerURL(cfg),
		ClientID:    clientID,
		Username:    username,
		Password:    password,
		TopicPrefix: topicRoot(cfg),
		ExpiresAt:   expiresAt,
	}
}

func CredentialForServer(cfg Config, now time.Time) *Credential {
	id := strings.TrimSpace(ServerID)
	if !cfg.Enabled || id == "" {
		return nil
	}
	expiresAt := now.Add(cfg.CredentialTTL).Unix()
	clientID := fmt.Sprintf("%s-%s", defaultString(cfg.ClientIDPrefix, "slan-device"), strings.ReplaceAll(id, "/", "-"))
	username := SystemUsername(cfg, id, expiresAt)
	password := sign(cfg.Secret, clientID, username, id)
	return &Credential{
		BrokerURL:   cfg.BrokerURL,
		ClientID:    clientID,
		Username:    username,
		Password:    password,
		TopicPrefix: topicRoot(cfg),
		ExpiresAt:   expiresAt,
	}
}

func publicBrokerURL(cfg Config) string {
	if strings.TrimSpace(cfg.PublicBrokerURL) != "" {
		return strings.TrimSpace(cfg.PublicBrokerURL)
	}
	return strings.TrimSpace(cfg.BrokerURL)
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
