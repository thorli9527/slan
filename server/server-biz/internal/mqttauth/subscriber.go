package mqttauth

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/slan/server/server-biz/configs"
)

// Subscribe publishes raw QoS0 MQTT PUBLISH payloads to handler until ctx is
// cancelled or the broker connection fails.
func Subscribe(ctx context.Context, cfg configs.MQTTConfig, clientID, username, password, topicFilter string, handler func(string, []byte)) error {
	if !cfg.Enabled {
		return nil
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
	subscribe, err := subscribePacket(1, topicFilter)
	if err != nil {
		return err
	}
	if _, err := conn.Write(subscribe); err != nil {
		return err
	}
	if err := readSubAck(conn, 1); err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Time{})
	done := make(chan struct{})
	defer close(done)
	go keepAlive(conn, done)
	for {
		select {
		case <-ctx.Done():
			_, _ = conn.Write([]byte{0xe0, 0x00})
			return ctx.Err()
		default:
		}
		header := []byte{0}
		if _, err := io.ReadFull(conn, header); err != nil {
			return err
		}
		remaining, err := readRemainingLength(conn)
		if err != nil {
			return err
		}
		body := make([]byte, remaining)
		if _, err := io.ReadFull(conn, body); err != nil {
			return err
		}
		switch header[0] & 0xf0 {
		case 0x30:
			topic, payload, err := parsePublish(header[0], body)
			if err == nil {
				handler(topic, payload)
			}
		case 0xc0:
			_, _ = conn.Write([]byte{0xd0, 0x00})
		}
	}
}

func keepAlive(conn net.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			_, _ = conn.Write([]byte{0xc0, 0x00})
		}
	}
}

func readConnAck(conn net.Conn) error {
	ack := make([]byte, 4)
	if _, err := io.ReadFull(conn, ack); err != nil {
		return err
	}
	if ack[0] != 0x20 || ack[1] != 0x02 || ack[3] != 0x00 {
		return fmt.Errorf("mqtt connect rejected")
	}
	return nil
}

func subscribePacket(packetID uint16, topicFilter string) ([]byte, error) {
	var variable bytes.Buffer
	_ = binary.Write(&variable, binary.BigEndian, packetID)
	if err := writeString(&variable, topicFilter); err != nil {
		return nil, err
	}
	variable.WriteByte(0x00)
	remaining, err := remainingLength(variable.Len())
	if err != nil {
		return nil, err
	}

	var packet bytes.Buffer
	packet.WriteByte(0x82)
	packet.Write(remaining)
	packet.Write(variable.Bytes())
	return packet.Bytes(), nil
}

func readSubAck(reader io.Reader, packetID uint16) error {
	header := []byte{0}
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}
	if header[0]&0xf0 != 0x90 {
		return fmt.Errorf("mqtt subscribe rejected")
	}
	remaining, err := readRemainingLength(reader)
	if err != nil {
		return err
	}
	body := make([]byte, remaining)
	if _, err := io.ReadFull(reader, body); err != nil {
		return err
	}
	if len(body) < 3 {
		return fmt.Errorf("mqtt subscribe rejected")
	}
	if binary.BigEndian.Uint16(body[:2]) != packetID {
		return fmt.Errorf("mqtt subscribe rejected")
	}
	for _, code := range body[2:] {
		if code == 0x80 {
			return fmt.Errorf("mqtt subscribe rejected")
		}
	}
	return nil
}

func readRemainingLength(reader io.Reader) (int, error) {
	multiplier := 1
	value := 0
	for i := 0; i < 4; i++ {
		var encoded [1]byte
		if _, err := io.ReadFull(reader, encoded[:]); err != nil {
			return 0, err
		}
		value += int(encoded[0]&127) * multiplier
		if encoded[0]&128 == 0 {
			return value, nil
		}
		multiplier *= 128
	}
	return 0, fmt.Errorf("malformed mqtt remaining length")
}

func parsePublish(header byte, body []byte) (string, []byte, error) {
	if len(body) < 2 {
		return "", nil, fmt.Errorf("mqtt publish too short")
	}
	topicLength := int(binary.BigEndian.Uint16(body[:2]))
	if len(body) < 2+topicLength {
		return "", nil, fmt.Errorf("mqtt publish topic truncated")
	}
	topic := string(body[2 : 2+topicLength])
	payloadOffset := 2 + topicLength
	qos := (header >> 1) & 0x03
	if qos > 0 {
		if len(body) < payloadOffset+2 {
			return "", nil, fmt.Errorf("mqtt publish packet id truncated")
		}
		payloadOffset += 2
	}
	payload := body[payloadOffset:]
	return topic, payload, nil
}
