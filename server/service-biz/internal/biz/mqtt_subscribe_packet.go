package biz

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

func deviceIDFromControlUpTopic(cfg MQTTConfig, topic string) string {
	deviceID, suffix := deviceIDAndSuffixFromDeviceTopic(cfg, topic)
	if suffix != "control/up" {
		return ""
	}
	return deviceID
}

func deviceIDAndSuffixFromDeviceTopic(cfg MQTTConfig, topic string) (string, string) {
	suffix := strings.TrimPrefix(trimTopic(topic), mqttTopicRoot(cfg)+"/devices/")
	parts := strings.Split(suffix, "/")
	if len(parts) < 2 || parts[0] == "" {
		return "", ""
	}
	return parts[0], strings.Join(parts[1:], "/")
}

func mqttSubscribePacket(packetID uint16, topicFilter string, qos byte) []byte {
	var variable bytes.Buffer
	_ = binary.Write(&variable, binary.BigEndian, packetID)
	variable.WriteByte(0x00)
	mqttWriteString(&variable, topicFilter)
	variable.WriteByte(qos)
	var packet bytes.Buffer
	packet.WriteByte(0x82)
	packet.Write(mqttRemainingLength(variable.Len()))
	packet.Write(variable.Bytes())
	return packet.Bytes()
}

func mqttReadSubAck(reader io.Reader, packetID uint16) error {
	header, body, err := mqttReadPacket(reader)
	if err != nil {
		return err
	}
	if header&0xf0 != 0x90 || len(body) < 4 || binary.BigEndian.Uint16(body[:2]) != packetID {
		return fmt.Errorf("mqtt subscribe rejected")
	}
	cursor := 2
	propLen, consumed, err := mqttReadRemainingLengthBytes(body[cursor:])
	if err != nil {
		return err
	}
	cursor += consumed + propLen
	if cursor >= len(body) {
		return fmt.Errorf("mqtt subscribe missing reason code")
	}
	for _, code := range body[cursor:] {
		if code > 2 {
			return fmt.Errorf("mqtt subscribe rejected code=%d", code)
		}
	}
	return nil
}

func mqttParsePublish(header byte, body []byte) (mqttIncomingPublish, error) {
	if len(body) < 3 {
		return mqttIncomingPublish{}, fmt.Errorf("mqtt publish too short")
	}
	topicLen := int(binary.BigEndian.Uint16(body[:2]))
	cursor := 2
	if len(body) < cursor+topicLen {
		return mqttIncomingPublish{}, fmt.Errorf("mqtt publish topic truncated")
	}
	topic := string(body[cursor : cursor+topicLen])
	cursor += topicLen
	qos := (header >> 1) & 0x03
	var packetID uint16
	if qos > 0 {
		if len(body) < cursor+2 {
			return mqttIncomingPublish{}, fmt.Errorf("mqtt publish packet id truncated")
		}
		packetID = binary.BigEndian.Uint16(body[cursor : cursor+2])
		cursor += 2
	}
	propLen, consumed, err := mqttReadRemainingLengthBytes(body[cursor:])
	if err != nil {
		return mqttIncomingPublish{}, err
	}
	cursor += consumed + propLen
	if len(body) < cursor {
		return mqttIncomingPublish{}, fmt.Errorf("mqtt publish properties truncated")
	}
	return mqttIncomingPublish{Topic: topic, Payload: body[cursor:], QoS: qos, PacketID: packetID}, nil
}

func mqttReadRemainingLengthBytes(body []byte) (int, int, error) {
	multiplier := 1
	value := 0
	for i, b := range body {
		value += int(b&127) * multiplier
		if b&128 == 0 {
			return value, i + 1, nil
		}
		multiplier *= 128
		if i >= 3 {
			break
		}
	}
	return 0, 0, fmt.Errorf("malformed mqtt remaining length")
}
