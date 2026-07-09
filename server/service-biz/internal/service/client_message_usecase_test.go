package service

import (
	"testing"

	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func TestTargetDeviceDownstreamTopic(t *testing.T) {
	topic := targetDeviceDownstreamTopic(
		mqttkit.Config{TopicPrefix: "slan"},
		" device-002 ",
	)
	if topic != "slan/devices/device-002/control/down" {
		t.Fatalf("unexpected target device downstream topic: %s", topic)
	}
}
