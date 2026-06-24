package app

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func newMQTTConfig() mqttkit.Config {
	cfg := mqttkit.DefaultConfig()

	if value, ok := envBool("SLAN_MQTT_ENABLED"); ok {
		cfg.Enabled = value
	}
	if value := envString("SLAN_MQTT_PUBLIC_BROKER_URL"); value != "" {
		cfg.BrokerURL = value
	} else if value := envString("SLAN_MQTT_BROKER_URL"); value != "" {
		cfg.BrokerURL = value
	}
	if value := envString("SLAN_MQTT_TOPIC_PREFIX"); value != "" {
		cfg.TopicPrefix = value
	}
	if value, ok := envDurationMilliseconds("SLAN_MQTT_CREDENTIAL_TTL_MILLISECONDS"); ok {
		cfg.CredentialTTL = value
	} else if value, ok := envDuration("SLAN_MQTT_CREDENTIAL_TTL"); ok {
		cfg.CredentialTTL = value
	}
	if value := envString("SLAN_MQTT_CLIENT_ID_PREFIX"); value != "" {
		cfg.ClientIDPrefix = value
	}
	if value := envString("SLAN_MQTT_USERNAME_PREFIX"); value != "" {
		cfg.UsernamePrefix = value
	}
	if value := envString("SLAN_MQTT_PASSWORD_SECRET"); value != "" {
		cfg.Secret = value
	}

	return cfg
}

func envString(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envBool(key string) (bool, bool) {
	value := envString(key)
	if value == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, false
	}
	return parsed, true
}

func envDuration(key string) (time.Duration, bool) {
	value := envString(key)
	if value == "" {
		return 0, false
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func envDurationMilliseconds(key string) (time.Duration, bool) {
	value := envString(key)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(parsed) * time.Millisecond, true
}
