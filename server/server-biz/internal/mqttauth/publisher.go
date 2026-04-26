package mqttauth

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/slan/server/server-biz/configs"
)

func PublishJSON(ctx context.Context, cfg configs.MQTTConfig, credentialClientID, credentialUsername, credentialPassword, topic string, payload any) error {
	if !cfg.Enabled {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return publish(ctx, cfg, credentialClientID, credentialUsername, credentialPassword, topic, body)
}

func publish(ctx context.Context, cfg configs.MQTTConfig, clientID, username, password, topic string, payload []byte) error {
	address, err := brokerAddress(cfg.BrokerURL)
	if err != nil {
		return err
	}
	timeout := time.Duration(cfg.PublishTimeoutMilliseconds) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(connectPacket(clientID, username, password)); err != nil {
		return err
	}
	if err := readConnAck(conn); err != nil {
		return err
	}
	if _, err := conn.Write(publishPacket(topic, payload)); err != nil {
		return err
	}
	_, _ = conn.Write([]byte{0xe0, 0x00})
	return nil
}

func brokerAddress(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("mqtt broker url missing host")
	}
	if parsed.Port() != "" {
		return parsed.Host, nil
	}
	return net.JoinHostPort(parsed.Hostname(), "1883"), nil
}

func connectPacket(clientID, username, password string) []byte {
	var variable bytes.Buffer
	writeString(&variable, "MQTT")
	variable.WriteByte(0x04)
	variable.WriteByte(0x02 | 0x80 | 0x40)
	_ = binary.Write(&variable, binary.BigEndian, uint16(30))
	writeString(&variable, clientID)
	writeString(&variable, username)
	writeString(&variable, password)

	var packet bytes.Buffer
	packet.WriteByte(0x10)
	packet.Write(remainingLength(variable.Len()))
	packet.Write(variable.Bytes())
	return packet.Bytes()
}

func publishPacket(topic string, payload []byte) []byte {
	var variable bytes.Buffer
	writeString(&variable, topic)
	variable.Write(payload)

	var packet bytes.Buffer
	packet.WriteByte(0x30)
	packet.Write(remainingLength(variable.Len()))
	packet.Write(variable.Bytes())
	return packet.Bytes()
}

func writeString(buf *bytes.Buffer, value string) {
	_ = binary.Write(buf, binary.BigEndian, uint16(len(value)))
	buf.WriteString(value)
}

func remainingLength(length int) []byte {
	var encoded []byte
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 128
		}
		encoded = append(encoded, digit)
		if length == 0 {
			return encoded
		}
	}
}
