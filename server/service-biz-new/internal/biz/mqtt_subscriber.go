package biz

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

type mqttIncomingPublish struct {
	Topic    string
	Payload  []byte
	QoS      byte
	PacketID uint16
}

type controlEnvelopeRaw struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	MessageID string          `json:"messageId,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type clientMessageUpPayload struct {
	MessageID      string         `json:"messageId,omitempty"`
	NetworkID      string         `json:"networkId"`
	FromDeviceID   string         `json:"fromDeviceId"`
	TargetDeviceID string         `json:"targetDeviceId"`
	Body           string         `json:"body"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

func (s *Server) startMQTTControlSubscriber() {
	if !s.mqtt.Enabled {
		return
	}
	go s.runMQTTControlSubscriber(context.Background())
}

func (s *Server) runMQTTControlSubscriber(ctx context.Context) {
	backoff := time.Second
	for {
		if err := s.runMQTTControlSubscriberOnce(ctx); err != nil {
			log.Printf("mqtt control subscriber disconnected: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 10*time.Second {
			backoff *= 2
		}
	}
}

func (s *Server) runMQTTControlSubscriberOnce(ctx context.Context) error {
	credential := serverMQTTCredential(s.mqtt, timeNow())
	if credential == nil {
		return nil
	}
	address, err := mqttBrokerAddress(s.mqtt.BrokerURL)
	if err != nil {
		return err
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	timeout := time.Duration(s.mqtt.PublishTimeoutMilliseconds) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	refreshMQTTDeadline(conn, timeout)
	if _, err := conn.Write(mqttConnectPacket(credential.ClientID+"-subscriber", credential.Username, credential.Password)); err != nil {
		return err
	}
	if err := mqttReadConnAck(conn); err != nil {
		return err
	}
	topicFilter := mqttTopicRoot(s.mqtt) + "/devices/+/control/up"
	if _, err := conn.Write(mqttSubscribePacket(1, topicFilter, mqttQoSExactlyOnce)); err != nil {
		return err
	}
	if err := mqttReadSubAck(conn, 1); err != nil {
		return err
	}
	log.Printf("mqtt control subscriber connected topic=%s", topicFilter)
	_ = conn.SetDeadline(time.Time{})
	lastPing := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		header, body, err := mqttReadPacket(conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if time.Since(lastPing) >= 15*time.Second {
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if _, err := conn.Write([]byte{0xc0, 0x00}); err != nil {
						return err
					}
					lastPing = time.Now()
				}
				continue
			}
			return err
		}
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		switch header & 0xf0 {
		case 0x30:
			publish, err := mqttParsePublish(header, body)
			if err != nil {
				return err
			}
			if publish.QoS == 1 {
				_, _ = conn.Write([]byte{0x40, 0x02, byte(publish.PacketID >> 8), byte(publish.PacketID)})
			}
			if publish.QoS == 2 {
				_, _ = conn.Write([]byte{0x50, 0x02, byte(publish.PacketID >> 8), byte(publish.PacketID)})
			}
			if err := s.handleMQTTControlUp(ctx, publish.Topic, publish.Payload); err != nil {
				log.Printf("mqtt control up rejected topic=%s: %v", publish.Topic, err)
			}
		case 0x60:
			if len(body) >= 2 {
				_, _ = conn.Write([]byte{0x70, 0x02, body[0], body[1]})
			}
		case 0xc0:
			_, _ = conn.Write([]byte{0xd0, 0x00})
		case 0xd0:
		default:
		}
	}
}

func (s *Server) handleMQTTControlUp(ctx context.Context, topic string, body []byte) error {
	fromTopicDeviceID := deviceIDFromControlUpTopic(s.mqtt, topic)
	if fromTopicDeviceID == "" {
		return fmt.Errorf("invalid control up topic")
	}
	var envelope controlEnvelopeRaw
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode control envelope: %w", err)
	}
	if envelope.Type != "client_message" {
		return nil
	}
	var payload clientMessageUpPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode client message payload: %w", err)
	}
	payload.NetworkID = strings.TrimSpace(payload.NetworkID)
	payload.FromDeviceID = strings.TrimSpace(payload.FromDeviceID)
	payload.TargetDeviceID = strings.TrimSpace(payload.TargetDeviceID)
	payload.Body = strings.TrimSpace(payload.Body)
	if payload.MessageID == "" {
		payload.MessageID = envelope.MessageID
	}
	if payload.FromDeviceID != fromTopicDeviceID {
		return fmt.Errorf("fromDeviceId does not match mqtt topic")
	}
	if payload.NetworkID == "" || payload.TargetDeviceID == "" || payload.Body == "" {
		return fmt.Errorf("networkId, targetDeviceId and body are required")
	}
	config, err := s.store.NetworkConfig(payload.NetworkID, payload.FromDeviceID)
	if err != nil {
		return err
	}
	allowedTarget := payload.TargetDeviceID == payload.FromDeviceID
	for _, peer := range config.Peers {
		if peer.DeviceID == payload.TargetDeviceID {
			allowedTarget = true
			break
		}
	}
	if !allowedTarget {
		return fmt.Errorf("target device is not active in the same network")
	}
	messageID := defaultString(payload.MessageID, fmt.Sprintf("client-msg-%d", timeNow().UnixNano()))
	payload.MessageID = messageID
	out := controlEnvelope{
		Type:      "client_message",
		MessageID: messageID,
		Payload:   payload,
	}
	credential := serverMQTTCredential(s.mqtt, timeNow())
	if credential == nil {
		return fmt.Errorf("server mqtt credential unavailable")
	}
	publishCtx, cancel := context.WithTimeout(ctx, time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
	defer cancel()
	return mqttPublishJSON(
		publishCtx,
		s.mqtt,
		credential.ClientID+"-"+messageID,
		credential.Username,
		credential.Password,
		mqttControlDownTopic(s.mqtt, payload.TargetDeviceID),
		out,
		mqttQoSExactlyOnce,
	)
}

func deviceIDFromControlUpTopic(cfg MQTTConfig, topic string) string {
	suffix := strings.TrimPrefix(trimTopic(topic), mqttTopicRoot(cfg)+"/devices/")
	parts := strings.Split(suffix, "/")
	if len(parts) != 3 || parts[1] != "control" || parts[2] != "up" {
		return ""
	}
	return parts[0]
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
