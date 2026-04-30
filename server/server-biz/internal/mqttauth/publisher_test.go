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
	_, err := publishPacket(strings.Repeat("a", 65536), []byte("payload"), defaultPublishOptions())
	if err == nil || !strings.Contains(err.Error(), "mqtt string too long") {
		t.Fatalf("expected long MQTT string error, got %v", err)
	}
}

func TestPublishPacketUsesQoS2(t *testing.T) {
	packet, err := publishPacket("slan/dev-1/control/down", []byte("payload"), defaultPublishOptions())
	if err != nil {
		t.Fatalf("publish packet: %v", err)
	}
	if len(packet) < 5 || packet[0] != 0x34 {
		t.Fatalf("expected qos2 publish header, got % x", packet)
	}
	topicLen := int(packet[2])<<8 | int(packet[3])
	packetIDOffset := 4 + topicLen
	if len(packet) < packetIDOffset+2 {
		t.Fatalf("publish packet truncated: % x", packet)
	}
	packetID := uint16(packet[packetIDOffset])<<8 | uint16(packet[packetIDOffset+1])
	if packetID != 1 {
		t.Fatalf("expected packet id 1, got %d", packetID)
	}
}

func TestPublishPacketSupportsQoS0(t *testing.T) {
	packet, err := publishPacket("slan/dev-1/state", []byte("payload"), PublishOptions{QoS: PublishQoSAtMostOnce})
	if err != nil {
		t.Fatalf("publish packet: %v", err)
	}
	if len(packet) < 5 || packet[0] != 0x30 {
		t.Fatalf("expected qos0 publish header, got % x", packet)
	}
	topicLen := int(packet[2])<<8 | int(packet[3])
	payloadOffset := 4 + topicLen
	if string(packet[payloadOffset:]) != "payload" {
		t.Fatalf("expected payload without packet id, got % x", packet[payloadOffset:])
	}
}

func TestPublishPacketRejectsUnsupportedQoS(t *testing.T) {
	_, err := publishPacket("slan/dev-1/state", []byte("payload"), PublishOptions{QoS: 1})
	if err == nil || !strings.Contains(err.Error(), "unsupported mqtt publish qos 1") {
		t.Fatalf("expected unsupported qos error, got %v", err)
	}
}

func TestValidatePublishOptionsAcceptsSupportedQoS(t *testing.T) {
	if err := validatePublishOptions(PublishOptions{QoS: PublishQoSAtMostOnce}); err != nil {
		t.Fatalf("qos0 should be accepted: %v", err)
	}
	if err := validatePublishOptions(PublishOptions{QoS: PublishQoSExactlyOnce}); err != nil {
		t.Fatalf("qos2 should be accepted: %v", err)
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
