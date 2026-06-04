package biz

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	mqttQoSAtMostOnce               byte = 0
	mqttQoSExactlyOnce              byte = 2
	mqttServerID                         = "service-biz"
	mqttControlSchemaVersion             = 1
	defaultControlMessageTTLSeconds      = 5 * 60
)

type MQTTConfig struct {
	Enabled                    bool
	BrokerURL                  string
	PublicBrokerURL            string
	UsernamePrefix             string
	PasswordSecret             string
	TopicPrefix                string
	CredentialTTLSeconds       int
	ControlMessageTTLSeconds   int
	PublishTimeoutMilliseconds int
}

type MQTTCredential struct {
	BrokerURL   string `json:"brokerUrl"`
	ClientID    string `json:"clientId"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	TopicPrefix string `json:"topicPrefix"`
	ExpiresAt   int64  `json:"expiresAt"`
}

type MQTTAuthOK struct {
	TenantID string            `json:"tenantId"`
	UserID   string            `json:"userId"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

type MQTTAuthResponse struct {
	OK     *MQTTAuthOK `json:"ok,omitempty"`
	Reject string      `json:"reject,omitempty"`
}

type MQTT5AuthResponse struct {
	Success *MQTTAuthOK      `json:"success,omitempty"`
	Failed  *MQTT5AuthFailed `json:"failed,omitempty"`
}

type MQTT5AuthFailed struct {
	Code string `json:"code"`
}

type mqttAuthResult struct {
	Principal string
	DeviceID  string
}

type controlEnvelope struct {
	Type          string `json:"type"`
	RequestID     string `json:"requestId,omitempty"`
	MessageID     string `json:"messageId,omitempty"`
	SchemaVersion int    `json:"schemaVersion"`
	CreatedAt     int64  `json:"createdAt"`
	ExpiresAt     int64  `json:"expiresAt"`
	Payload       any    `json:"payload,omitempty"`
}

func mqttConfigFromEnv() MQTTConfig {
	cfg := MQTTConfig{
		Enabled:                    false,
		BrokerURL:                  "mqtt://127.0.0.1:1883",
		PublicBrokerURL:            "mqtt://127.0.0.1:1883",
		UsernamePrefix:             "slan",
		PasswordSecret:             "dev-mqtt-secret",
		TopicPrefix:                "slan",
		CredentialTTLSeconds:       int((30 * 24 * time.Hour).Seconds()),
		ControlMessageTTLSeconds:   defaultControlMessageTTLSeconds,
		PublishTimeoutMilliseconds: 15000,
	}
	if value := os.Getenv("SLAN_MQTT_ENABLED"); value != "" {
		cfg.Enabled = strings.EqualFold(value, "true") || value == "1"
	}
	if value := os.Getenv("SLAN_MQTT_BROKER_URL"); value != "" {
		cfg.BrokerURL = value
	}
	if value := os.Getenv("SLAN_MQTT_PUBLIC_BROKER_URL"); value != "" {
		cfg.PublicBrokerURL = value
	}
	if value := os.Getenv("SLAN_MQTT_USERNAME_PREFIX"); value != "" {
		cfg.UsernamePrefix = value
	}
	if value := os.Getenv("SLAN_MQTT_PASSWORD_SECRET"); value != "" {
		cfg.PasswordSecret = value
	}
	if value := os.Getenv("SLAN_MQTT_TOPIC_PREFIX"); value != "" {
		cfg.TopicPrefix = value
	}
	if value := os.Getenv("SLAN_MQTT_CREDENTIAL_TTL_SECONDS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.CredentialTTLSeconds = parsed
		}
	}
	if value := os.Getenv("SLAN_MQTT_CONTROL_MESSAGE_TTL_SECONDS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.ControlMessageTTLSeconds = parsed
		}
	}
	if value := os.Getenv("SLAN_MQTT_PUBLISH_TIMEOUT_MILLISECONDS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.PublishTimeoutMilliseconds = parsed
		}
	}
	return cfg
}
