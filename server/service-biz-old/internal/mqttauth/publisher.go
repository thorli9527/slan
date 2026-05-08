package mqttauth

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/slan/server/server-biz/configs"
)

const maxMQTTRemainingLength = 268435455

const (
	PublishQoSAtMostOnce  byte = 0
	PublishQoSExactlyOnce byte = 2
)

type PublishOptions struct {
	QoS byte
}

func defaultPublishOptions() PublishOptions {
	return PublishOptions{QoS: PublishQoSExactlyOnce}
}

func PublishJSON(ctx context.Context, cfg configs.MQTTConfig, credentialClientID, credentialUsername, credentialPassword, topic string, payload any) error {
	return PublishJSONWithOptions(ctx, cfg, credentialClientID, credentialUsername, credentialPassword, topic, payload, defaultPublishOptions())
}

func PublishJSONWithOptions(ctx context.Context, cfg configs.MQTTConfig, credentialClientID, credentialUsername, credentialPassword, topic string, payload any, options PublishOptions) error {
	if !cfg.Enabled {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return publish(ctx, cfg, credentialClientID, credentialUsername, credentialPassword, topic, body, options)
}

func publish(ctx context.Context, cfg configs.MQTTConfig, clientID, username, password, topic string, payload []byte, options PublishOptions) error {
	if err := validatePublishOptions(options); err != nil {
		return err
	}
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
	connect, err := connectPacket(clientID, username, password)
	if err != nil {
		return err
	}
	refreshConnDeadline(conn, timeout)
	if _, err := conn.Write(connect); err != nil {
		return err
	}
	refreshConnDeadline(conn, timeout)
	if err := readConnAck(conn); err != nil {
		return err
	}
	publish, err := publishPacket(topic, payload, options)
	if err != nil {
		return err
	}
	refreshConnDeadline(conn, timeout)
	if _, err := conn.Write(publish); err != nil {
		return err
	}
	if options.QoS == PublishQoSExactlyOnce {
		if err := completeQoS2Publish(conn, 1, timeout); err != nil {
			return err
		}
	}
	refreshConnDeadline(conn, timeout)
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
	variable.WriteByte(0x05)
	variable.WriteByte(0x02 | 0x80 | 0x40)
	_ = binary.Write(&variable, binary.BigEndian, uint16(30))
	variable.WriteByte(0x00)
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

func publishPacket(topic string, payload []byte, options PublishOptions) ([]byte, error) {
	if err := validatePublishOptions(options); err != nil {
		return nil, err
	}
	var variable bytes.Buffer
	if err := writeString(&variable, topic); err != nil {
		return nil, err
	}
	if options.QoS > 0 {
		_ = binary.Write(&variable, binary.BigEndian, uint16(1))
	}
	variable.WriteByte(0x00)
	variable.Write(payload)
	remaining, err := remainingLength(variable.Len())
	if err != nil {
		return nil, err
	}

	var packet bytes.Buffer
	packet.WriteByte(0x30 | (options.QoS << 1))
	packet.Write(remaining)
	packet.Write(variable.Bytes())
	return packet.Bytes(), nil
}

func validatePublishOptions(options PublishOptions) error {
	if options.QoS != PublishQoSAtMostOnce && options.QoS != PublishQoSExactlyOnce {
		return fmt.Errorf("unsupported mqtt publish qos %d", options.QoS)
	}
	return nil
}

func completeQoS2Publish(conn net.Conn, packetID uint16, timeout time.Duration) error {
	refreshConnDeadline(conn, timeout)
	header, body, err := readPacket(conn)
	if err != nil {
		return err
	}
	if header&0xf0 != 0x50 || len(body) < 2 || binary.BigEndian.Uint16(body[:2]) != packetID {
		return fmt.Errorf("mqtt pubrec rejected")
	}
	pubrel := []byte{0x62, 0x02, byte(packetID >> 8), byte(packetID)}
	refreshConnDeadline(conn, timeout)
	if _, err := conn.Write(pubrel); err != nil {
		return err
	}
	refreshConnDeadline(conn, timeout)
	header, body, err = readPacket(conn)
	if err != nil {
		return err
	}
	if header&0xf0 != 0x70 || len(body) < 2 || binary.BigEndian.Uint16(body[:2]) != packetID {
		return fmt.Errorf("mqtt pubcomp rejected")
	}
	return nil
}

func refreshConnDeadline(conn net.Conn, timeout time.Duration) {
	if timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}
}

func readPacket(reader io.Reader) (byte, []byte, error) {
	header := []byte{0}
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, nil, err
	}
	remaining, err := readRemainingLength(reader)
	if err != nil {
		return 0, nil, err
	}
	body := make([]byte, remaining)
	if _, err := io.ReadFull(reader, body); err != nil {
		return 0, nil, err
	}
	return header[0], body, nil
}

func readConnAck(conn net.Conn) error {
	header, body, err := readPacket(conn)
	if err != nil {
		return err
	}
	if header&0xf0 != 0x20 || len(body) < 2 || body[1] != 0x00 {
		return fmt.Errorf("mqtt connect rejected")
	}
	return nil
}

func decodeRemainingLength(data []byte) (int, int, error) {
	multiplier := 1
	value := 0
	for i := 0; i < 4 && i < len(data); i++ {
		value += int(data[i]&127) * multiplier
		if data[i]&128 == 0 {
			return value, i + 1, nil
		}
		multiplier *= 128
	}
	return 0, 0, fmt.Errorf("malformed mqtt remaining length")
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
