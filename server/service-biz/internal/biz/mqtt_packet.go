package biz

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

func mqttBrokerAddress(raw string) (string, error) {
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

func mqttConnectPacket(clientID, username, password string) []byte {
	var variable bytes.Buffer
	mqttWriteString(&variable, "MQTT")
	variable.WriteByte(0x05)
	variable.WriteByte(0x02 | 0x80 | 0x40)
	_ = binary.Write(&variable, binary.BigEndian, uint16(30))
	variable.WriteByte(0x00)
	mqttWriteString(&variable, clientID)
	mqttWriteString(&variable, username)
	mqttWriteString(&variable, password)
	var packet bytes.Buffer
	packet.WriteByte(0x10)
	packet.Write(mqttRemainingLength(variable.Len()))
	packet.Write(variable.Bytes())
	return packet.Bytes()
}

func mqttPublishPacket(topic string, payload []byte, qos byte, packetID uint16) ([]byte, error) {
	if qos != mqttQoSAtMostOnce && qos != mqttQoSExactlyOnce {
		return nil, fmt.Errorf("unsupported mqtt qos %d", qos)
	}
	var variable bytes.Buffer
	mqttWriteString(&variable, topic)
	if qos > 0 {
		_ = binary.Write(&variable, binary.BigEndian, packetID)
	}
	variable.WriteByte(0x00)
	variable.Write(payload)
	var packet bytes.Buffer
	packet.WriteByte(0x30 | (qos << 1))
	packet.Write(mqttRemainingLength(variable.Len()))
	packet.Write(variable.Bytes())
	return packet.Bytes(), nil
}

func mqttCompleteQoS2(conn net.Conn, packetID uint16, timeout time.Duration) error {
	refreshMQTTDeadline(conn, timeout)
	header, body, err := mqttReadPacket(conn)
	if err != nil {
		return err
	}
	if header&0xf0 != 0x50 || len(body) < 2 || binary.BigEndian.Uint16(body[:2]) != packetID {
		return fmt.Errorf("mqtt pubrec rejected")
	}
	refreshMQTTDeadline(conn, timeout)
	if _, err := conn.Write([]byte{0x62, 0x02, byte(packetID >> 8), byte(packetID)}); err != nil {
		return err
	}
	refreshMQTTDeadline(conn, timeout)
	header, body, err = mqttReadPacket(conn)
	if err != nil {
		return err
	}
	if header&0xf0 != 0x70 || len(body) < 2 || binary.BigEndian.Uint16(body[:2]) != packetID {
		return fmt.Errorf("mqtt pubcomp rejected")
	}
	return nil
}

func mqttReadConnAck(reader io.Reader) error {
	header, body, err := mqttReadPacket(reader)
	if err != nil {
		return err
	}
	if header&0xf0 != 0x20 || len(body) < 2 || body[1] != 0x00 {
		return fmt.Errorf("mqtt connect rejected")
	}
	return nil
}

func mqttReadPacket(reader io.Reader) (byte, []byte, error) {
	header := []byte{0}
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, nil, err
	}
	remaining, err := mqttReadRemainingLength(reader)
	if err != nil {
		return 0, nil, err
	}
	body := make([]byte, remaining)
	if _, err := io.ReadFull(reader, body); err != nil {
		return 0, nil, err
	}
	return header[0], body, nil
}

func mqttReadRemainingLength(reader io.Reader) (int, error) {
	multiplier := 1
	value := 0
	for i := 0; i < 4; i++ {
		buf := []byte{0}
		if _, err := io.ReadFull(reader, buf); err != nil {
			return 0, err
		}
		value += int(buf[0]&127) * multiplier
		if buf[0]&128 == 0 {
			return value, nil
		}
		multiplier *= 128
	}
	return 0, fmt.Errorf("malformed mqtt remaining length")
}

func mqttWriteString(buf *bytes.Buffer, value string) {
	_ = binary.Write(buf, binary.BigEndian, uint16(len(value)))
	buf.WriteString(value)
}

func mqttRemainingLength(length int) []byte {
	out := make([]byte, 0, 4)
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		out = append(out, digit)
		if length == 0 {
			return out
		}
	}
}

func refreshMQTTDeadline(conn net.Conn, timeout time.Duration) {
	if timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}
}
