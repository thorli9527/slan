package biz

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	mqttQoSAtMostOnce               byte = 0
	mqttQoSExactlyOnce              byte = 2
	mqttServerID                         = "service-biz"
	mqttControlSchemaVersion             = 1
	defaultControlMessageTTLSeconds      = 5 * 60
)

type MQTTConfig struct {
	Enabled                    bool
	BrokerURL                  string
	PublicBrokerURL            string
	UsernamePrefix             string
	PasswordSecret             string
	TopicPrefix                string
	CredentialTTLSeconds       int
	ControlMessageTTLSeconds   int
	PublishTimeoutMilliseconds int
}

type MQTTCredential struct {
	BrokerURL   string `json:"brokerUrl"`
	ClientID    string `json:"clientId"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	TopicPrefix string `json:"topicPrefix"`
	ExpiresAt   int64  `json:"expiresAt"`
}

type MQTTAuthOK struct {
	TenantID string            `json:"tenantId"`
	UserID   string            `json:"userId"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

type MQTTAuthResponse struct {
	OK     *MQTTAuthOK `json:"ok,omitempty"`
	Reject string      `json:"reject,omitempty"`
}

type MQTT5AuthResponse struct {
	Success *MQTTAuthOK      `json:"success,omitempty"`
	Failed  *MQTT5AuthFailed `json:"failed,omitempty"`
}

type MQTT5AuthFailed struct {
	Code string `json:"code"`
}

type mqttAuthResult struct {
	Principal string
	DeviceID  string
}

type controlEnvelope struct {
	Type          string `json:"type"`
	RequestID     string `json:"requestId,omitempty"`
	MessageID     string `json:"messageId,omitempty"`
	SchemaVersion int    `json:"schemaVersion"`
	CreatedAt     int64  `json:"createdAt"`
	ExpiresAt     int64  `json:"expiresAt"`
	Payload       any    `json:"payload,omitempty"`
}

func mqttConfigFromEnv() MQTTConfig {
	cfg := MQTTConfig{
		Enabled:                    false,
		BrokerURL:                  "mqtt://127.0.0.1:1883",
		PublicBrokerURL:            "mqtt://127.0.0.1:1883",
		UsernamePrefix:             "slan",
		PasswordSecret:             "dev-mqtt-secret",
		TopicPrefix:                "slan/v1",
		CredentialTTLSeconds:       int((30 * 24 * time.Hour).Seconds()),
		ControlMessageTTLSeconds:   defaultControlMessageTTLSeconds,
		PublishTimeoutMilliseconds: 15000,
	}
	if value := os.Getenv("SLAN_MQTT_ENABLED"); value != "" {
		cfg.Enabled = strings.EqualFold(value, "true") || value == "1"
	}
	if value := os.Getenv("SLAN_MQTT_BROKER_URL"); value != "" {
		cfg.BrokerURL = value
	}
	if value := os.Getenv("SLAN_MQTT_PUBLIC_BROKER_URL"); value != "" {
		cfg.PublicBrokerURL = value
	}
	if value := os.Getenv("SLAN_MQTT_USERNAME_PREFIX"); value != "" {
		cfg.UsernamePrefix = value
	}
	if value := os.Getenv("SLAN_MQTT_PASSWORD_SECRET"); value != "" {
		cfg.PasswordSecret = value
	}
	if value := os.Getenv("SLAN_MQTT_TOPIC_PREFIX"); value != "" {
		cfg.TopicPrefix = value
	}
	if value := os.Getenv("SLAN_MQTT_CREDENTIAL_TTL_SECONDS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.CredentialTTLSeconds = parsed
		}
	}
	if value := os.Getenv("SLAN_MQTT_CONTROL_MESSAGE_TTL_SECONDS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.ControlMessageTTLSeconds = parsed
		}
	}
	if value := os.Getenv("SLAN_MQTT_PUBLISH_TIMEOUT_MILLISECONDS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.PublishTimeoutMilliseconds = parsed
		}
	}
	return cfg
}

func deviceMQTTCredential(cfg MQTTConfig, deviceID string, now time.Time) *MQTTCredential {
	deviceID = strings.TrimSpace(deviceID)
	if !cfg.Enabled || deviceID == "" {
		return nil
	}
	expiresAt := now.Add(time.Duration(cfg.CredentialTTLSeconds) * time.Second).Unix()
	clientID := mqttClientID(cfg, deviceID)
	username := mqttUsername(cfg, deviceID, expiresAt)
	return &MQTTCredential{
		BrokerURL:   mqttPublicBrokerURL(cfg),
		ClientID:    clientID,
		Username:    username,
		Password:    mqttPassword(cfg, clientID, username, deviceID),
		TopicPrefix: mqttDeviceTopicPrefix(cfg, deviceID),
		ExpiresAt:   expiresAt,
	}
}

func serverMQTTCredential(cfg MQTTConfig, now time.Time) *MQTTCredential {
	if !cfg.Enabled {
		return nil
	}
	expiresAt := now.Add(time.Duration(cfg.CredentialTTLSeconds) * time.Second).Unix()
	clientID := mqttClientID(cfg, "server")
	username := mqttSystemUsername(cfg, mqttServerID, expiresAt)
	return &MQTTCredential{
		BrokerURL:   cfg.BrokerURL,
		ClientID:    clientID,
		Username:    username,
		Password:    mqttPassword(cfg, clientID, username, mqttServerID),
		TopicPrefix: mqttTopicRoot(cfg),
		ExpiresAt:   expiresAt,
	}
}

func validateMQTTCredential(cfg MQTTConfig, clientID, username, givenPassword string, now time.Time) (mqttAuthResult, bool) {
	if deviceID, ok := validateDeviceMQTTCredential(cfg, clientID, username, givenPassword, now); ok {
		return mqttAuthResult{Principal: "device", DeviceID: deviceID}, true
	}
	if validateServerMQTTCredential(cfg, clientID, username, givenPassword, now) {
		return mqttAuthResult{Principal: "server"}, true
	}
	return mqttAuthResult{}, false
}

func validateDeviceMQTTCredential(cfg MQTTConfig, clientID, username, givenPassword string, now time.Time) (string, bool) {
	if !cfg.Enabled {
		return "", false
	}
	deviceID, expiresAt, ok := parseMQTTUsername(cfg, username)
	if !ok || expiresAt < now.Unix() {
		return "", false
	}
	baseClientID := mqttClientID(cfg, deviceID)
	if clientID != baseClientID && !strings.HasPrefix(clientID, baseClientID+"-") {
		return "", false
	}
	return deviceID, hmac.Equal([]byte(mqttPassword(cfg, clientID, username, deviceID)), []byte(givenPassword)) ||
		hmac.Equal([]byte(mqttPassword(cfg, baseClientID, username, deviceID)), []byte(givenPassword))
}

func validateServerMQTTCredential(cfg MQTTConfig, clientID, username, givenPassword string, now time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	expiresAt, ok := parseMQTTSystemUsername(cfg, username, mqttServerID)
	if !ok || expiresAt < now.Unix() {
		return false
	}
	baseClientID := mqttClientID(cfg, "server")
	if clientID != baseClientID && !strings.HasPrefix(clientID, baseClientID+"-") {
		return false
	}
	return hmac.Equal([]byte(mqttPassword(cfg, clientID, username, mqttServerID)), []byte(givenPassword)) ||
		hmac.Equal([]byte(mqttPassword(cfg, baseClientID, username, mqttServerID)), []byte(givenPassword))
}

func mqttAllowTopicAccess(cfg MQTTConfig, principal, deviceID, topic string, subscribe bool) bool {
	topic = trimTopic(topic)
	if topic == "" {
		return false
	}
	if principal == "server" {
		if subscribe {
			return topic == mqttTopicRoot(cfg)+"/devices/#" ||
				isControlUpTopic(cfg, topic) ||
				topic == mqttServerControlUpTopic(cfg)
		}
		return isControlDownTopic(cfg, topic) || isNetworkBroadcastTopic(cfg, topic)
	}
	if principal != "device" || strings.TrimSpace(deviceID) == "" {
		return false
	}
	devicePrefix := mqttDeviceTopicPrefix(cfg, deviceID)
	if subscribe {
		return topic == devicePrefix+"/control/down" ||
			isNetworkBroadcastTopic(cfg, topic)
	}
	return topic == devicePrefix+"/control/up" ||
		topic == devicePrefix+"/control/ack" ||
		topic == devicePrefix+"/heartbeat" ||
		topic == devicePrefix+"/runtime" ||
		topic == devicePrefix+"/runtime-state" ||
		isDeviceNetworkStateTopic(cfg, deviceID, topic)
}

func mqttDeviceTopicPrefix(cfg MQTTConfig, deviceID string) string {
	return mqttTopicRoot(cfg) + "/devices/" + strings.TrimSpace(deviceID)
}

func mqttControlDownTopic(cfg MQTTConfig, deviceID string) string {
	return mqttDeviceTopicPrefix(cfg, deviceID) + "/control/down"
}

func mqttNetworkBroadcastTopic(cfg MQTTConfig, networkID string) string {
	return mqttTopicRoot(cfg) + "/networks/" + strings.TrimSpace(networkID) + "/broadcast"
}

func mqttNetworkMemberStateTopic(cfg MQTTConfig, networkID, deviceID string) string {
	return mqttTopicRoot(cfg) + "/networks/" + strings.TrimSpace(networkID) + "/members/" + strings.TrimSpace(deviceID) + "/state"
}

func mqttServerControlUpTopic(cfg MQTTConfig) string {
	return mqttTopicRoot(cfg) + "/server/control/up"
}

func isControlUpTopic(cfg MQTTConfig, topic string) bool {
	suffix := strings.TrimPrefix(topic, mqttTopicRoot(cfg)+"/devices/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] == "control" && parts[2] == "up"
}

func isControlDownTopic(cfg MQTTConfig, topic string) bool {
	suffix := strings.TrimPrefix(topic, mqttTopicRoot(cfg)+"/devices/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] == "control" && parts[2] == "down"
}

func isNetworkBroadcastTopic(cfg MQTTConfig, topic string) bool {
	suffix := strings.TrimPrefix(topic, mqttTopicRoot(cfg)+"/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 3 && parts[0] == "networks" && parts[1] != "" && parts[2] == "broadcast"
}

func isDeviceNetworkStateTopic(cfg MQTTConfig, deviceID, topic string) bool {
	suffix := strings.TrimPrefix(topic, mqttTopicRoot(cfg)+"/networks/")
	parts := strings.Split(suffix, "/")
	return len(parts) == 4 && parts[0] != "" && parts[1] == "members" && parts[2] == deviceID && parts[3] == "state"
}

func mqttClientID(cfg MQTTConfig, id string) string {
	return sanitizeDNSLabel(cfg.UsernamePrefix) + "-" + strings.ReplaceAll(strings.TrimSpace(id), "/", "-")
}

func mqttUsername(cfg MQTTConfig, id string, expiresAt int64) string {
	return fmt.Sprintf("%s:%s:%d", strings.TrimSpace(cfg.UsernamePrefix), strings.TrimSpace(id), expiresAt)
}

func mqttSystemUsername(cfg MQTTConfig, id string, expiresAt int64) string {
	return fmt.Sprintf("%s:system:%s:%d", strings.TrimSpace(cfg.UsernamePrefix), strings.TrimSpace(id), expiresAt)
}

func parseMQTTUsername(cfg MQTTConfig, username string) (string, int64, bool) {
	parts := strings.Split(strings.TrimSpace(username), ":")
	if len(parts) != 3 || parts[0] != strings.TrimSpace(cfg.UsernamePrefix) {
		return "", 0, false
	}
	expiresAt, err := strconv.ParseInt(parts[2], 10, 64)
	return parts[1], expiresAt, err == nil && parts[1] != ""
}

func parseMQTTSystemUsername(cfg MQTTConfig, username, expectedID string) (int64, bool) {
	parts := strings.Split(strings.TrimSpace(username), ":")
	if len(parts) != 4 || parts[0] != strings.TrimSpace(cfg.UsernamePrefix) || parts[1] != "system" || parts[2] != expectedID {
		return 0, false
	}
	expiresAt, err := strconv.ParseInt(parts[3], 10, 64)
	return expiresAt, err == nil
}

func mqttPassword(cfg MQTTConfig, clientID, username, id string) string {
	mac := hmac.New(sha256.New, []byte(cfg.PasswordSecret))
	mac.Write([]byte(clientID))
	mac.Write([]byte{0})
	mac.Write([]byte(username))
	mac.Write([]byte{0})
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

func mqttPublicBrokerURL(cfg MQTTConfig) string {
	if strings.TrimSpace(cfg.PublicBrokerURL) != "" {
		return strings.TrimSpace(cfg.PublicBrokerURL)
	}
	return strings.TrimSpace(cfg.BrokerURL)
}

func publishControlMQTT(ctx context.Context, cfg MQTTConfig, deviceID, msgType string, payload any) error {
	messageID := mqttMessageID("msg")
	return publishControlMQTTWithMessageID(ctx, cfg, deviceID, msgType, messageID, payload)
}

func publishControlMQTTWithMessageID(ctx context.Context, cfg MQTTConfig, deviceID, msgType, messageID string, payload any) error {
	credential := serverMQTTCredential(cfg, time.Now())
	if credential == nil {
		return errUnavailable
	}
	topic := mqttControlDownTopic(cfg, deviceID)
	return mqttPublishJSON(ctx, cfg, credential.ClientID+"-"+messageID, credential.Username, credential.Password, topic, newControlEnvelope(cfg, msgType, messageID, payload), mqttQoSExactlyOnce)
}

func mqttMessageID(prefix string) string {
	token, err := secureTokenHex(12)
	if err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + token
}

func newControlEnvelope(cfg MQTTConfig, msgType, messageID string, payload any) controlEnvelope {
	now := timeNow().Unix()
	ttl := cfg.ControlMessageTTLSeconds
	if ttl <= 0 {
		ttl = defaultControlMessageTTLSeconds
	}
	return controlEnvelope{
		Type:          msgType,
		MessageID:     strings.TrimSpace(messageID),
		SchemaVersion: mqttControlSchemaVersion,
		CreatedAt:     now,
		ExpiresAt:     now + int64(ttl),
		Payload:       payload,
	}
}

func mqttPublishJSON(ctx context.Context, cfg MQTTConfig, clientID, username, password, topic string, payload any, qos byte) error {
	if !cfg.Enabled {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return mqttPublish(ctx, cfg, clientID, username, password, topic, body, qos)
}

func mqttPublish(ctx context.Context, cfg MQTTConfig, clientID, username, password, topic string, payload []byte, qos byte) error {
	address, err := mqttBrokerAddress(cfg.BrokerURL)
	if err != nil {
		return err
	}
	timeout := time.Duration(cfg.PublishTimeoutMilliseconds) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	refreshMQTTDeadline(conn, timeout)
	if _, err := conn.Write(mqttConnectPacket(clientID, username, password)); err != nil {
		return err
	}
	refreshMQTTDeadline(conn, timeout)
	if err := mqttReadConnAck(conn); err != nil {
		return err
	}
	packet, err := mqttPublishPacket(topic, payload, qos, 1)
	if err != nil {
		return err
	}
	refreshMQTTDeadline(conn, timeout)
	if _, err := conn.Write(packet); err != nil {
		return err
	}
	if qos == mqttQoSExactlyOnce {
		if err := mqttCompleteQoS2(conn, 1, timeout); err != nil {
			return err
		}
	}
	_, _ = conn.Write([]byte{0xe0, 0x00})
	return nil
}

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

func trimTopic(topic string) string {
	return strings.Trim(strings.TrimSpace(topic), "/")
}

func mqttTopicRoot(cfg MQTTConfig) string {
	root := trimTopic(cfg.TopicPrefix)
	if root == "" {
		return "slan/v1"
	}
	return root
}
