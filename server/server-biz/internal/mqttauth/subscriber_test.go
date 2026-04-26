package mqttauth

import (
	"bytes"
	"encoding/binary"
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

func publishBody(topic string, packetID []byte, payload []byte) []byte {
	var body bytes.Buffer
	_ = binary.Write(&body, binary.BigEndian, uint16(len(topic)))
	body.WriteString(topic)
	body.Write(packetID)
	body.Write(payload)
	return body.Bytes()
}
