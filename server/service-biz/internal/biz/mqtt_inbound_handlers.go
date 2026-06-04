package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

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

type deviceRuntimeUpPayload struct {
	DeviceID        string `json:"deviceId,omitempty"`
	ActiveNetworkID string `json:"activeNetworkId,omitempty"`
	NetworkEnabled  bool   `json:"networkEnabled,omitempty"`
	VirtualIP       string `json:"virtualIp,omitempty"`
	SignedIn        bool   `json:"signedIn,omitempty"`
	RxBytesTotal    uint64 `json:"rxBytesTotal,omitempty"`
	TxBytesTotal    uint64 `json:"txBytesTotal,omitempty"`
	ReportedAtMs    int64  `json:"reportedAtMs,omitempty"`
}

type controlAckUpPayload struct {
	TaskID        string `json:"taskId"`
	DeliveryID    string `json:"deliveryId"`
	Action        string `json:"action"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
	ProcessedAtMs int64  `json:"processedAtMs,omitempty"`
}

type pathHealthReportUpPayload struct {
	NetworkID         string `json:"networkId"`
	PeerNodeID        string `json:"peerNodeId,omitempty"`
	PathType          string `json:"pathType,omitempty"`
	ActivePath        string `json:"activePath,omitempty"`
	RelayTransport    string `json:"relayTransport,omitempty"`
	Endpoint          string `json:"endpoint,omitempty"`
	DerpNodeID        string `json:"derpNodeId,omitempty"`
	ObservedRttMs     int64  `json:"observedRttMs,omitempty"`
	PacketLossPpm     int64  `json:"packetLossPpm,omitempty"`
	PathScore         int64  `json:"pathScore,omitempty"`
	RelayMtu          int    `json:"relayMtu,omitempty"`
	MaxFramePayload   int    `json:"maxFramePayload,omitempty"`
	TicketExpiresAt   string `json:"ticketExpiresAt,omitempty"`
	TicketExpiresInMs int64  `json:"ticketExpiresInMs,omitempty"`
	TicketRenewDue    bool   `json:"ticketRenewDue,omitempty"`
	PathDowngrades    int64  `json:"pathDowngrades,omitempty"`
	PathUpgrades      int64  `json:"pathUpgrades,omitempty"`
	LastPathChange    string `json:"lastPathChange,omitempty"`
	SampledAtMs       int64  `json:"sampledAtMs,omitempty"`
}

type wirePathHealthRequest struct {
	PeerID string          `json:"peerId"`
	Probes []wirePathProbe `json:"probes"`
}

type wirePathProbe struct {
	Path       string `json:"path"`
	Reachable  bool   `json:"reachable"`
	RTTMs      int    `json:"rttMs,omitempty"`
	LossPPM    int    `json:"lossPpm,omitempty"`
	MTU        int    `json:"mtu,omitempty"`
	ObservedAt int64  `json:"observedAt,omitempty"`
}

func (s *Server) handleMQTTDevicePublish(ctx context.Context, topic string, body []byte) error {
	s.ensureServices()
	deviceID, suffix := deviceIDAndSuffixFromDeviceTopic(s.mqtt, topic)
	if deviceID == "" {
		return fmt.Errorf("invalid device topic")
	}
	switch suffix {
	case "control/up":
		return s.handleMQTTControlUp(ctx, topic, body)
	case "control/ack":
		return s.handleMQTTControlAck(deviceID, body)
	case "heartbeat", "runtime", "runtime-state":
		return s.handleMQTTRuntimeUp(deviceID, body)
	default:
		return nil
	}
}

func (s *Server) handleMQTTRuntimeUp(topicDeviceID string, body []byte) error {
	var payload deviceRuntimeUpPayload
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("decode device runtime payload: %w", err)
		}
	}
	if payload.DeviceID != "" && strings.TrimSpace(payload.DeviceID) != topicDeviceID {
		return fmt.Errorf("runtime deviceId does not match mqtt topic")
	}
	result := s.services.MQTT.ReportDeviceRuntime(topicDeviceID, payload.NetworkEnabled, payload.RxBytesTotal, payload.TxBytesTotal)
	if result.NetworkEnabledChanged {
		s.services.Audit.Record(AuditEvent{
			ActorType:    "device",
			ActorID:      result.DeviceID,
			Action:       "client_network.runtime_state_changed",
			ResourceType: "device",
			ResourceID:   result.DeviceID,
			Status:       "succeeded",
			Details: map[string]string{
				"networkEnabled": boolString(result.NetworkEnabled),
				"networkIds":     strings.Join(result.NetworkIDs, ","),
				"rxBytesTotal":   fmt.Sprintf("%d", payload.RxBytesTotal),
				"txBytesTotal":   fmt.Sprintf("%d", payload.TxBytesTotal),
			},
			CreatedAt: result.ChangedAt,
		})
		for _, networkID := range result.NetworkIDs {
			s.notifyDeviceNetworkPresence(networkID, result.DeviceID, result.NetworkEnabled, result.ChangedAt)
		}
	}
	return nil
}

func (s *Server) handleMQTTControlAck(topicDeviceID string, body []byte) error {
	var payload controlAckUpPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("decode control ack payload: %w", err)
	}
	payload.DeliveryID = strings.TrimSpace(payload.DeliveryID)
	payload.TaskID = strings.TrimSpace(payload.TaskID)
	payload.Action = strings.TrimSpace(payload.Action)
	payload.Status = strings.TrimSpace(payload.Status)
	payload.Error = strings.TrimSpace(payload.Error)
	if payload.DeliveryID == "" || payload.Status == "" {
		return fmt.Errorf("deliveryId and status are required")
	}
	delivery, err := s.services.MQTT.RecordAck(
		topicDeviceID,
		payload.DeliveryID,
		payload.TaskID,
		payload.Action,
		payload.Status,
		payload.Error,
		payload.ProcessedAtMs,
		timeNow().Unix(),
	)
	if err != nil {
		if errors.Is(err, errConflict) {
			log.Printf("mqtt control ack ignored expired delivery device=%s deliveryId=%s taskId=%s status=%s action=%s", topicDeviceID, payload.DeliveryID, payload.TaskID, payload.Status, payload.Action)
			return nil
		}
		return err
	}
	if isNetworkControlAction(payload.Action) || isNetworkControlAction(delivery.Action) {
		auditStatus := "succeeded"
		if !strings.EqualFold(payload.Status, "succeeded") {
			auditStatus = "failed"
		}
		s.services.Audit.Record(AuditEvent{
			ActorType:    "device",
			ActorID:      delivery.DeviceID,
			Action:       "client_network.control_ack",
			ResourceType: "mqtt_control_delivery",
			ResourceID:   delivery.DeliveryID,
			Status:       auditStatus,
			Details: map[string]string{
				"taskId":     delivery.TaskID,
				"mqttAction": delivery.Action,
				"ackStatus":  delivery.Status,
				"error":      delivery.Error,
			},
		})
	}
	log.Printf("mqtt control ack recorded device=%s deliveryId=%s taskId=%s status=%s action=%s", delivery.DeviceID, delivery.DeliveryID, delivery.TaskID, delivery.Status, delivery.Action)
	return nil
}

func isNetworkControlAction(action string) bool {
	switch strings.TrimSpace(action) {
	case "enableNetwork", "disableNetwork":
		return true
	default:
		return false
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
	if envelope.Type == "endpoint_report" {
		return s.handleMQTTEndpointReport(fromTopicDeviceID, envelope.Payload)
	}
	if envelope.Type == "path_health_report" {
		return s.handleMQTTPathHealthReport(ctx, fromTopicDeviceID, envelope.Payload)
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
	log.Printf("mqtt client_message received network=%s from=%s target=%s messageId=%s", payload.NetworkID, payload.FromDeviceID, payload.TargetDeviceID, defaultString(payload.MessageID, envelope.MessageID))
	config, err := s.services.MQTT.NetworkConfig(payload.NetworkID, payload.FromDeviceID)
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
		for _, membership := range s.services.MQTT.ListNetworkDevices(payload.NetworkID) {
			if membership.DeviceID == payload.TargetDeviceID && membership.Enabled {
				allowedTarget = true
				break
			}
		}
	}
	if !allowedTarget {
		return fmt.Errorf("target device is not in the same network")
	}
	messageID := defaultString(payload.MessageID, mqttMessageID("client-msg"))
	payload.MessageID = messageID
	out := newControlEnvelope(s.mqtt, "client_message", messageID, payload)
	credential := serverMQTTCredential(s.mqtt, timeNow())
	if credential == nil {
		return fmt.Errorf("server mqtt credential unavailable")
	}
	publishCtx, cancel := context.WithTimeout(ctx, time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
	defer cancel()
	if err := mqttPublishJSON(
		publishCtx,
		s.mqtt,
		credential.ClientID+"-"+messageID,
		credential.Username,
		credential.Password,
		mqttControlDownTopic(s.mqtt, payload.TargetDeviceID),
		out,
		mqttQoSExactlyOnce,
	); err != nil {
		return err
	}
	log.Printf("mqtt client_message forwarded network=%s from=%s target=%s messageId=%s", payload.NetworkID, payload.FromDeviceID, payload.TargetDeviceID, messageID)
	return nil
}

func (s *Server) handleMQTTEndpointReport(fromTopicDeviceID string, raw json.RawMessage) error {
	var payload struct {
		NetworkID string           `json:"networkId"`
		NodeID    string           `json:"nodeId"`
		NATType   string           `json:"natType"`
		Endpoints []DeviceEndpoint `json:"endpoints"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode endpoint report payload: %w", err)
	}
	networkID := strings.TrimSpace(payload.NetworkID)
	nodeID := strings.TrimSpace(payload.NodeID)
	if nodeID != "" && nodeID != "node-"+fromTopicDeviceID {
		return fmt.Errorf("endpoint report nodeId does not match mqtt topic")
	}
	if networkID == "" {
		return fmt.Errorf("endpoint report networkId is required")
	}
	changed, err := s.services.MQTT.ReportDeviceEndpoint(networkID, fromTopicDeviceID, payload.Endpoints)
	if err != nil {
		return err
	}
	log.Printf("mqtt endpoint_report recorded network=%s device=%s endpoints=%d", networkID, fromTopicDeviceID, len(payload.Endpoints))
	if changed {
		s.notifyNetworkConfigChanged(networkID, "device_endpoint_reported", "device_endpoint", "update", fromTopicDeviceID, fromTopicDeviceID)
	}
	return nil
}

func (s *Server) handleMQTTPathHealthReport(ctx context.Context, fromTopicDeviceID string, raw json.RawMessage) error {
	var payload pathHealthReportUpPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode path health report payload: %w", err)
	}
	payload.NetworkID = strings.TrimSpace(payload.NetworkID)
	payload.PeerNodeID = strings.TrimSpace(payload.PeerNodeID)
	payload.PathType = strings.TrimSpace(payload.PathType)
	payload.ActivePath = strings.TrimSpace(payload.ActivePath)
	payload.RelayTransport = strings.TrimSpace(payload.RelayTransport)
	payload.Endpoint = strings.TrimSpace(payload.Endpoint)
	payload.DerpNodeID = strings.TrimSpace(payload.DerpNodeID)
	if payload.NetworkID == "" {
		return fmt.Errorf("path health report networkId is required")
	}
	pathType := payload.PathType
	if pathType == "" {
		pathType = payload.ActivePath
	}
	if pathType == "" {
		return fmt.Errorf("path health report pathType or activePath is required")
	}
	if payload.PeerNodeID != "" && !strings.HasPrefix(payload.PeerNodeID, "node-") {
		return fmt.Errorf("path health report peerNodeId is invalid")
	}
	if _, err := s.services.MQTT.NetworkConfig(payload.NetworkID, fromTopicDeviceID); err != nil {
		return err
	}
	if err := forwardPathHealthReportToWire(ctx, payload.NetworkID, fromTopicDeviceID, payload, pathType); err != nil {
		return err
	}
	log.Printf(
		"mqtt path_health_report accepted network=%s device=%s peerNode=%s path=%s relayTransport=%s endpoint=%s derpNode=%s rttMs=%d lossPpm=%d score=%d mtu=%d framePayload=%d ticketRenewDue=%t sampledAtMs=%d",
		payload.NetworkID,
		fromTopicDeviceID,
		payload.PeerNodeID,
		pathType,
		payload.RelayTransport,
		payload.Endpoint,
		payload.DerpNodeID,
		payload.ObservedRttMs,
		payload.PacketLossPpm,
		payload.PathScore,
		payload.RelayMtu,
		payload.MaxFramePayload,
		payload.TicketRenewDue,
		payload.SampledAtMs,
	)
	return nil
}

func forwardPathHealthReportToWire(ctx context.Context, networkID, deviceID string, payload pathHealthReportUpPayload, pathType string) error {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SLAN_BIZ_WIRE_INTERNAL_URL")), "/")
	if baseURL == "" || !wirePathHealthForwardable(pathType) {
		return nil
	}
	body, err := json.Marshal(wirePathHealthRequest{
		PeerID: wirePeerID(networkID, deviceID),
		Probes: []wirePathProbe{{
			Path:       pathType,
			Reachable:  payload.PathScore < 10_000 && payload.PacketLossPpm < 1_000_000,
			RTTMs:      nonNegativeInt64ToInt(payload.ObservedRttMs),
			LossPPM:    nonNegativeInt64ToInt(payload.PacketLossPpm),
			MTU:        payload.RelayMtu,
			ObservedAt: payload.SampledAtMs,
		}},
	})
	if err != nil {
		return fmt.Errorf("encode wire path health request: %w", err)
	}
	timeout := 3 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	forwardCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(forwardCtx, http.MethodPost, baseURL+"/peers/path-health", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create wire path health request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(os.Getenv("SLAN_INTERNAL_WIRE_TOKEN")); token != "" {
		req.Header.Set("X-Slan-Internal-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("forward wire path health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("forward wire path health returned %d", resp.StatusCode)
	}
	return nil
}

func wirePathHealthForwardable(pathType string) bool {
	switch strings.TrimSpace(pathType) {
	case "lan_udp", "ipv6_udp", "direct_udp", "relay_udp", "derp_tcp_tls_443":
		return true
	default:
		return false
	}
}

func nonNegativeInt64ToInt(value int64) int {
	if value <= 0 {
		return 0
	}
	maxInt := int64(^uint(0) >> 1)
	if value > maxInt {
		return int(maxInt)
	}
	return int(value)
}
