package service

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func NewNetworkEventPublisher(cfg mqttkit.Config) NetworkEventPublisher {
	if !cfg.Enabled || strings.TrimSpace(cfg.BrokerURL) == "" {
		return nil
	}
	return NewMqttNetworkEventPublisher(cfg)
}

func mqttBrokerURLForClient(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse mqtt broker url: %w", err)
	}
	switch u.Scheme {
	case "mqtt", "tcp":
		u.Scheme = "tcp"
	case "mqtts", "ssl", "tls":
		u.Scheme = "ssl"
	default:
		return "", fmt.Errorf("unsupported mqtt broker scheme %q", u.Scheme)
	}
	return u.String(), nil
}
