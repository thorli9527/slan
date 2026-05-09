package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type authResponse struct {
	AccessToken string `json:"accessToken"`
}

type networkHome struct {
	ActiveNetwork *network `json:"activeNetwork"`
	OwnedNetwork  *network `json:"ownedNetwork"`
}

type network struct {
	NetworkID string `json:"networkId"`
}

type device struct {
	DeviceID string          `json:"deviceId"`
	MQTT     *mqttCredential `json:"mqtt"`
}

type mqttCredential struct {
	BrokerURL   string `json:"brokerUrl"`
	ClientID    string `json:"clientId"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	TopicPrefix string `json:"topicPrefix"`
}

type clientMessageResponse struct {
	MessageID      string `json:"messageId"`
	NetworkID      string `json:"networkId"`
	FromDeviceID   string `json:"fromDeviceId"`
	TargetDeviceID string `json:"targetDeviceId"`
}

type mqttPublish struct {
	topic    string
	payload  []byte
	qos      byte
	packetID uint16
}

func main() {
	var bizURL string
	var email string
	var password string
	var timeout time.Duration
	flag.StringVar(&bizURL, "biz-url", envDefault("SLAN_BIZ_URL", "http://127.0.0.1:28080"), "server-biz base URL")
	flag.StringVar(&email, "email", "", "test user email; defaults to unique smoke user")
	flag.StringVar(&password, "password", "Password123!", "test user password")
	flag.DurationVar(&timeout, "timeout", 8*time.Second, "MQTT receive timeout")
	flag.Parse()

	if email == "" {
		email = fmt.Sprintf("client-message-smoke-%d@example.test", time.Now().UnixNano())
	}
	bizURL = strings.TrimRight(bizURL, "/")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	token := register(ctx, bizURL, email, password)
	mac := registerDevice(ctx, bizURL, token, "smoke-mac-"+uniqueSuffix(), "macos")
	ios := registerDevice(ctx, bizURL, token, "smoke-ios-"+uniqueSuffix(), "ios")
	if ios.MQTT == nil {
		fail("target device returned no MQTT credential")
	}
	networkID := activeNetworkID(ctx, bizURL, token)
	body := "hello-from-smoke-" + uniqueSuffix()
	messageCh := make(chan map[string]any, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- subscribeOne(ctx, *ios.MQTT, ios.MQTT.TopicPrefix+"/control/down", messageCh)
	}()
	time.Sleep(300 * time.Millisecond)

	response := sendClientMessage(ctx, bizURL, token, networkID, mac.DeviceID, ios.DeviceID, body)
	select {
	case message := <-messageCh:
		payload, ok := message["payload"].(map[string]any)
		if !ok {
			fail("client_message payload missing: %#v", message)
		}
		if payload["messageId"] != response.MessageID {
			fail("messageId mismatch: mqtt=%v http=%s", payload["messageId"], response.MessageID)
		}
		if payload["fromDeviceId"] != mac.DeviceID || payload["targetDeviceId"] != ios.DeviceID {
			fail("device mismatch in mqtt payload: %#v", payload)
		}
		if payload["body"] != body {
			fail("body mismatch in mqtt payload: %#v", payload)
		}
		fmt.Printf("clientMessageMqttSmoke: ok email=%s networkId=%s from=%s target=%s messageId=%s\n", email, networkID, mac.DeviceID, ios.DeviceID, response.MessageID)
	case err := <-errCh:
		fail("mqtt subscribe failed before message: %v", err)
	case <-ctx.Done():
		fail("timed out waiting for MQTT client_message: %v", ctx.Err())
	}
}

func register(ctx context.Context, bizURL, email, password string) string {
	var out authResponse
	postJSON(ctx, bizURL+"/auth/register", "", map[string]any{
		"email":    email,
		"password": password,
	}, &out)
	if strings.TrimSpace(out.AccessToken) == "" {
		fail("register returned empty access token")
	}
	return out.AccessToken
}

func registerDevice(ctx context.Context, bizURL, token, deviceID, platform string) device {
	var out device
	postJSON(ctx, bizURL+"/devices/register", token, map[string]any{
		"deviceId":      deviceID,
		"name":          deviceID,
		"platform":      platform,
		"deviceVersion": "smoke",
		"publicKey":     "smoke-public-key-" + deviceID,
	}, &out)
	if out.DeviceID == "" {
		fail("register device %s returned empty deviceId", deviceID)
	}
	return out
}

func activeNetworkID(ctx context.Context, bizURL, token string) string {
	var home networkHome
	getJSON(ctx, bizURL+"/networks/home", token, &home)
	if home.ActiveNetwork != nil && home.ActiveNetwork.NetworkID != "" {
		return home.ActiveNetwork.NetworkID
	}
	if home.OwnedNetwork != nil && home.OwnedNetwork.NetworkID != "" {
		return home.OwnedNetwork.NetworkID
	}
	fail("network home returned no active/owned network")
	return ""
}

func sendClientMessage(ctx context.Context, bizURL, token, networkID, fromDeviceID, targetDeviceID, body string) clientMessageResponse {
	var out clientMessageResponse
	postJSON(ctx, bizURL+"/networks/"+url.PathEscape(networkID)+"/devices/"+url.PathEscape(targetDeviceID)+"/messages", token, map[string]any{
		"fromDeviceId": fromDeviceID,
		"body":         body,
		"metadata": map[string]any{
			"smoke": true,
		},
	}, &out)
	if out.MessageID == "" {
		fail("send client message returned empty messageId")
	}
	return out
}

func subscribeOne(ctx context.Context, credential mqttCredential, topicFilter string, messageCh chan<- map[string]any) error {
	address, err := brokerAddress(credential.BrokerURL)
	if err != nil {
		return err
	}
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := conn.Write(connectPacket(credential.ClientID+"-smoke", credential.Username, credential.Password)); err != nil {
		return err
	}
	if err := readConnAck(conn); err != nil {
		return err
	}
	if _, err := conn.Write(subscribePacket(1, topicFilter)); err != nil {
		return err
	}
	if err := readSubAck(conn, 1); err != nil {
		return err
	}
	for {
		header, body, err := readPacket(conn)
		if err != nil {
			return err
		}
		switch header & 0xf0 {
		case 0x30:
			publish, err := parsePublish(header, body)
			if err != nil {
				return err
			}
			var message map[string]any
			if err := json.Unmarshal(publish.payload, &message); err != nil {
				return err
			}
			if err := ackPublish(conn, publish); err != nil {
				return err
			}
			if message["type"] != "client_message" {
				continue
			}
			messageCh <- message
			return nil
		case 0xc0:
			_, _ = conn.Write([]byte{0xd0, 0x00})
		}
	}
}

func postJSON(ctx context.Context, url, token string, body any, out any) {
	payload, err := json.Marshal(body)
	if err != nil {
		fail("encode request %s: %v", url, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		fail("build request %s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	doJSON(req, token, out)
}

func getJSON(ctx context.Context, url, token string, out any) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fail("build request %s: %v", url, err)
	}
	doJSON(req, token, out)
}

func doJSON(req *http.Request, token string, out any) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fail("%s %s: HTTP %d: %s", req.Method, req.URL, resp.StatusCode, string(body))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			fail("decode %s %s response: %v body=%s", req.Method, req.URL, err, string(body))
		}
	}
}

func brokerAddress(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Host == "" {
		return "", errors.New("mqtt broker url missing host")
	}
	if parsed.Port() != "" {
		return parsed.Host, nil
	}
	return net.JoinHostPort(parsed.Hostname(), "1883"), nil
}

func connectPacket(clientID, username, password string) []byte {
	var variable bytes.Buffer
	writeString(&variable, "MQTT")
	variable.WriteByte(0x05)
	variable.WriteByte(0x02 | 0x80 | 0x40)
	_ = binary.Write(&variable, binary.BigEndian, uint16(30))
	variable.WriteByte(0x00)
	writeString(&variable, clientID)
	writeString(&variable, username)
	writeString(&variable, password)
	return packet(0x10, variable.Bytes())
}

func subscribePacket(packetID uint16, topicFilter string) []byte {
	var variable bytes.Buffer
	_ = binary.Write(&variable, binary.BigEndian, packetID)
	variable.WriteByte(0x00)
	writeString(&variable, topicFilter)
	variable.WriteByte(0x02)
	return packet(0x82, variable.Bytes())
}

func readConnAck(reader io.Reader) error {
	header, body, err := readPacket(reader)
	if err != nil {
		return err
	}
	if header&0xf0 != 0x20 || len(body) < 2 || body[1] != 0x00 {
		return fmt.Errorf("mqtt connect rejected")
	}
	return nil
}

func readSubAck(reader io.Reader, packetID uint16) error {
	header, body, err := readPacket(reader)
	if err != nil {
		return err
	}
	if header&0xf0 != 0x90 || len(body) < 4 || binary.BigEndian.Uint16(body[:2]) != packetID {
		return fmt.Errorf("mqtt subscribe rejected")
	}
	code := body[len(body)-1]
	if code != 0x02 {
		return fmt.Errorf("mqtt subscribe rejected code=%d", code)
	}
	return nil
}

func readPacket(reader io.Reader) (byte, []byte, error) {
	var header [1]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
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

func parsePublish(header byte, body []byte) (mqttPublish, error) {
	if len(body) < 2 {
		return mqttPublish{}, fmt.Errorf("mqtt publish too short")
	}
	topicLength := int(binary.BigEndian.Uint16(body[:2]))
	if len(body) < 2+topicLength {
		return mqttPublish{}, fmt.Errorf("mqtt publish topic truncated")
	}
	topic := string(body[2 : 2+topicLength])
	cursor := 2 + topicLength
	qos := (header >> 1) & 0x03
	var packetID uint16
	if qos > 0 {
		if len(body) < cursor+2 {
			return mqttPublish{}, fmt.Errorf("mqtt publish packet id truncated")
		}
		packetID = binary.BigEndian.Uint16(body[cursor : cursor+2])
		cursor += 2
	}
	propertyLength, propertyBytes, err := decodeRemainingLength(body[cursor:])
	if err != nil {
		return mqttPublish{}, err
	}
	cursor += propertyBytes + propertyLength
	if cursor > len(body) {
		return mqttPublish{}, fmt.Errorf("mqtt publish properties truncated")
	}
	return mqttPublish{topic: topic, payload: body[cursor:], qos: qos, packetID: packetID}, nil
}

func ackPublish(conn net.Conn, publish mqttPublish) error {
	switch publish.qos {
	case 0:
		return nil
	case 1:
		_, err := conn.Write(packetIDPacket(0x40, publish.packetID))
		return err
	case 2:
		if _, err := conn.Write(packetIDPacket(0x50, publish.packetID)); err != nil {
			return err
		}
		for {
			header, body, err := readPacket(conn)
			if err != nil {
				return err
			}
			if header&0xf0 == 0x60 && len(body) >= 2 && binary.BigEndian.Uint16(body[:2]) == publish.packetID {
				_, err := conn.Write(packetIDPacket(0x70, publish.packetID))
				return err
			}
			if header&0xf0 == 0xc0 {
				_, _ = conn.Write([]byte{0xd0, 0x00})
			}
		}
	default:
		return fmt.Errorf("invalid mqtt qos %d", publish.qos)
	}
}

func packet(typeAndFlags byte, body []byte) []byte {
	out := []byte{typeAndFlags}
	out = append(out, encodeRemainingLength(len(body))...)
	out = append(out, body...)
	return out
}

func packetIDPacket(typeAndFlags byte, packetID uint16) []byte {
	return []byte{typeAndFlags, 0x02, byte(packetID >> 8), byte(packetID)}
}

func writeString(buffer *bytes.Buffer, value string) {
	bytesValue := []byte(value)
	_ = binary.Write(buffer, binary.BigEndian, uint16(len(bytesValue)))
	buffer.Write(bytesValue)
}

func readRemainingLength(reader io.Reader) (int, error) {
	var multiplier = 1
	var value int
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

func decodeRemainingLength(data []byte) (int, int, error) {
	var multiplier = 1
	var value int
	for i := 0; i < 4 && i < len(data); i++ {
		value += int(data[i]&127) * multiplier
		if data[i]&128 == 0 {
			return value, i + 1, nil
		}
		multiplier *= 128
	}
	return 0, 0, fmt.Errorf("malformed mqtt remaining length")
}

func encodeRemainingLength(length int) []byte {
	var out []byte
	for {
		encoded := byte(length % 128)
		length /= 128
		if length > 0 {
			encoded |= 128
		}
		out = append(out, encoded)
		if length == 0 {
			return out
		}
	}
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "clientMessageMqttSmoke: "+format+"\n", args...)
	os.Exit(1)
}
