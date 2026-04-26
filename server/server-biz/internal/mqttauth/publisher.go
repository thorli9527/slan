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

const maxMQTTRemainingLength = 268435455

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
	connect, err := connectPacket(clientID, username, password)
	if err != nil {
		return err
	}
	if _, err := conn.Write(connect); err != nil {
		return err
	}
	if err := readConnAck(conn); err != nil {
		return err
	}
	publish, err := publishPacket(topic, payload)
	if err != nil {
		return err
	}
	if _, err := conn.Write(publish); err != nil {
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

func connectPacket(clientID, username, password string) ([]byte, error) {
	var variable bytes.Buffer
	if err := writeString(&variable, "MQTT"); err != nil {
		return nil, err
	}
	variable.WriteByte(0x04)
	variable.WriteByte(0x02 | 0x80 | 0x40)
	_ = binary.Write(&variable, binary.BigEndian, uint16(30))
	if err := writeString(&variable, clientID); err != nil {
		return nil, err
	}
	if err := writeString(&variable, username); err != nil {
		return nil, err
	}
	if err := writeString(&variable, password); err != nil {
		return nil, err
	}
	remaining, err := remainingLength(variable.Len())
	if err != nil {
		return nil, err
	}

	var packet bytes.Buffer
	packet.WriteByte(0x10)
	packet.Write(remaining)
	packet.Write(variable.Bytes())
	return packet.Bytes(), nil
}

func publishPacket(topic string, payload []byte) ([]byte, error) {
	var variable bytes.Buffer
	if err := writeString(&variable, topic); err != nil {
		return nil, err
	}
	variable.Write(payload)
	remaining, err := remainingLength(variable.Len())
	if err != nil {
		return nil, err
	}

	var packet bytes.Buffer
	packet.WriteByte(0x30)
	packet.Write(remaining)
	packet.Write(variable.Bytes())
	return packet.Bytes(), nil
}

func writeString(buf *bytes.Buffer, value string) error {
	if len(value) > 65535 {
		return fmt.Errorf("mqtt string too long")
	}
	_ = binary.Write(buf, binary.BigEndian, uint16(len(value)))
	buf.WriteString(value)
	return nil
}

func remainingLength(length int) ([]byte, error) {
	if length < 0 || length > maxMQTTRemainingLength {
		return nil, fmt.Errorf("mqtt remaining length out of range")
	}
	var encoded []byte
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 128
		}
		encoded = append(encoded, digit)
		if length == 0 {
			return encoded, nil
		}
	}
}
