package mqttauth

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestParsePublishQoS0(t *testing.T) {
	body := publishBody("slan/devices/dev-1/networks/net-1/state", nil, []byte(`{"ok":true}`))
	topic, payload, err := parsePublish(0x30, body)
	if err != nil {
		t.Fatalf("parse publish: %v", err)
	}
	if topic != "slan/devices/dev-1/networks/net-1/state" || string(payload) != `{"ok":true}` {
		t.Fatalf("unexpected topic=%s payload=%s", topic, payload)
	}
}

func TestParsePublishQoS1SkipsPacketID(t *testing.T) {
	body := publishBody("slan/devices/dev-1/networks/net-1/state", []byte{0x00, 0x07}, []byte(`{"ok":true}`))
	topic, payload, err := parsePublish(0x32, body)
	if err != nil {
		t.Fatalf("parse publish: %v", err)
	}
	if topic != "slan/devices/dev-1/networks/net-1/state" || string(payload) != `{"ok":true}` {
		t.Fatalf("unexpected topic=%s payload=%s", topic, payload)
	}
}

func TestReadSubAckAcceptsQoS0(t *testing.T) {
	if err := readSubAck(bytes.NewReader([]byte{0x90, 0x03, 0x00, 0x01, 0x00}), 1); err != nil {
		t.Fatalf("read suback: %v", err)
	}
}

func TestReadSubAckRejectsFailureCode(t *testing.T) {
	err := readSubAck(bytes.NewReader([]byte{0x90, 0x03, 0x00, 0x01, 0x80}), 1)
	if err == nil || !strings.Contains(err.Error(), "subscribe rejected") {
		t.Fatalf("expected rejected suback, got %v", err)
	}
}

func TestReadSubAckRejectsPacketIDMismatch(t *testing.T) {
	err := readSubAck(bytes.NewReader([]byte{0x90, 0x03, 0x00, 0x02, 0x00}), 1)
	if err == nil || !strings.Contains(err.Error(), "subscribe rejected") {
		t.Fatalf("expected rejected suback, got %v", err)
	}
}

func TestReadSubAckRejectsWrongPacketType(t *testing.T) {
	err := readSubAck(bytes.NewReader([]byte{0x30, 0x00}), 1)
	if err == nil || !strings.Contains(err.Error(), "subscribe rejected") {
		t.Fatalf("expected rejected suback, got %v", err)
	}
}

func TestSubscribePacketRejectsLongTopicFilter(t *testing.T) {
	_, err := subscribePacket(1, strings.Repeat("a", 65536))
	if err == nil || !strings.Contains(err.Error(), "mqtt string too long") {
		t.Fatalf("expected long MQTT string error, got %v", err)
	}
}

func publishBody(topic string, packetID []byte, payload []byte) []byte {
	var body bytes.Buffer
	_ = binary.Write(&body, binary.BigEndian, uint16(len(topic)))
	body.WriteString(topic)
	body.Write(packetID)
	body.Write(payload)
	return body.Bytes()
}
