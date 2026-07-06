package mqttkit

import "time"

type Config struct {
	Enabled         bool
	BrokerURL       string
	PublicBrokerURL string
	TopicPrefix     string
	CredentialTTL   time.Duration
	ClientIDPrefix  string
	UsernamePrefix  string
	Secret          string
}

func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		BrokerURL:       "mqtt://127.0.0.1:1883",
		PublicBrokerURL: "mqtt://127.0.0.1:1883",
		TopicPrefix:     "slan",
		CredentialTTL:   24 * time.Hour,
		ClientIDPrefix:  "slan-device",
		UsernamePrefix:  "device",
		Secret:          "slan-dev-secret",
	}
}
