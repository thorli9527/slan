package mqttauth

import (
	"strings"
	"testing"
)

func TestBrokerAddressDefaultsMQTTPort(t *testing.T) {
	address, err := brokerAddress("mqtt://bifromq")
	if err != nil {
		t.Fatalf("broker address: %v", err)
	}
	if address != "bifromq:1883" {
		t.Fatalf("unexpected broker address: %s", address)
	}
}

func TestBrokerAddressKeepsExplicitPort(t *testing.T) {
	address, err := brokerAddress(" mqtt://127.0.0.1:2883 ")
	if err != nil {
		t.Fatalf("broker address: %v", err)
	}
	if address != "127.0.0.1:2883" {
		t.Fatalf("unexpected broker address: %s", address)
	}
}

func TestBrokerAddressHandlesIPv6DefaultPort(t *testing.T) {
	address, err := brokerAddress("mqtt://[::1]")
	if err != nil {
		t.Fatalf("broker address: %v", err)
	}
	if address != "[::1]:1883" {
		t.Fatalf("unexpected broker address: %s", address)
	}
}

func TestBrokerAddressRejectsMissingHost(t *testing.T) {
	if _, err := brokerAddress("mqtt://"); err == nil {
		t.Fatal("expected missing host to fail")
	}
}

func TestConnectPacketRejectsLongClientID(t *testing.T) {
	_, err := connectPacket(strings.Repeat("a", 65536), "user", "pass")
	if err == nil || !strings.Contains(err.Error(), "mqtt string too long") {
		t.Fatalf("expected long MQTT string error, got %v", err)
	}
}

func TestPublishPacketRejectsLongTopic(t *testing.T) {
	_, err := publishPacket(strings.Repeat("a", 65536), []byte("payload"))
	if err == nil || !strings.Contains(err.Error(), "mqtt string too long") {
		t.Fatalf("expected long MQTT string error, got %v", err)
	}
}

func TestRemainingLengthRejectsOutOfRange(t *testing.T) {
	_, err := remainingLength(maxMQTTRemainingLength + 1)
	if err == nil || !strings.Contains(err.Error(), "remaining length out of range") {
		t.Fatalf("expected remaining length error, got %v", err)
	}
}

func TestRemainingLengthEncodesMultiByteValue(t *testing.T) {
	encoded, err := remainingLength(321)
	if err != nil {
		t.Fatalf("remaining length: %v", err)
	}
	if len(encoded) != 2 || encoded[0] != 0xc1 || encoded[1] != 0x02 {
		t.Fatalf("unexpected remaining length encoding: % x", encoded)
	}
}
