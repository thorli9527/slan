package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func (s MQTTWebhookService) Authenticate(_ context.Context, input MQTTAuthInput) (MQTTAuthView, error) {
	result, ok := mqttkit.ValidateCredential(s.Config, input.ClientID, input.Username, input.Password, currentTime(s.Now))
	if !ok {
		return MQTTAuthView{Allowed: false}, nil
	}
	userID := result.DeviceID
	if result.Principal == "server" {
		userID = mqttkit.ServerID
	}
	return MQTTAuthView{
		Allowed:   true,
		TenantID:  "slan",
		UserID:    userID,
		Principal: result.Principal,
		DeviceID:  result.DeviceID,
	}, nil
}

func (s MQTTWebhookService) CheckACL(ctx context.Context, input MQTTCheckInput) (bool, error) {
	principal := input.Principal
	deviceID := input.DeviceID
	userID := input.UserID
	if principal == "" {
		if userID == mqttkit.ServerID {
			principal = "server"
		} else if userID != "" {
			principal = "device"
			deviceID = userID
		}
	}
	allowed := input.Connect || mqttkit.AllowTopicAccess(s.Config, principal, deviceID, input.Topic, input.Subscribe)
	if allowed && principal == "device" && mqttkit.IsNetworkTopic(s.Config, input.Topic) {
		items, err := s.Networks.ListNetworkDevices(ctx, mqttkit.NetworkIDFromTopic(s.Config, input.Topic))
		if err != nil {
			return false, err
		}
		allowed = false
		for _, item := range items {
			if item.DeviceID == deviceID && item.Enabled && item.Status == "active" {
				allowed = true
				break
			}
		}
	}
	return allowed, nil
}

func (s MQTTWebhookService) ReportEndpoint(ctx context.Context, input MQTTEndpointReportInput) (bool, error) {
	input = normalizeMQTTEndpointReportInput(input)
	if input.NetworkID == "" || input.DeviceID == "" {
		return false, ErrInvalidArgument
	}
	if input.NodeID != "" && input.NodeID != "node-"+input.DeviceID {
		return false, ErrInvalidArgument
	}
	items, err := s.Networks.ListNetworkDevices(ctx, input.NetworkID)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item.DeviceID != input.DeviceID {
			continue
		}
		if !item.Enabled || item.Status != "active" {
			return false, ErrNotFound
		}
		updated := item
		updated.Endpoints = modelDeviceEndpoints(input.Endpoints)
		updated.NATType = input.NATType
		updated.UpdatedAt = currentTime(s.Now).Unix()
		if err := s.Networks.SaveNetworkDevice(ctx, updated); err != nil {
			return false, err
		}
		return deviceEndpointsChanged(item.Endpoints, updated.Endpoints), nil
	}
	return false, ErrNotFound
}

func normalizeMQTTEndpointReportInput(input MQTTEndpointReportInput) MQTTEndpointReportInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.NodeID = strings.TrimSpace(input.NodeID)
	input.NATType = strings.TrimSpace(input.NATType)
	endpoints := make([]DeviceEndpointView, 0, len(input.Endpoints))
	for _, item := range input.Endpoints {
		item.Type = strings.TrimSpace(item.Type)
		item.Address = strings.TrimSpace(item.Address)
		if item.Address == "" {
			continue
		}
		if item.Type == "" {
			item.Type = "direct_udp"
		}
		endpoints = append(endpoints, item)
	}
	input.Endpoints = endpoints
	return input
}

func modelDeviceEndpoints(items []DeviceEndpointView) []model.DeviceEndpoint {
	out := make([]model.DeviceEndpoint, 0, len(items))
	for _, item := range items {
		out = append(out, model.DeviceEndpoint{
			Type:      item.Type,
			Address:   item.Address,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return out
}

func deviceEndpointsChanged(previous, next []model.DeviceEndpoint) bool {
	if len(previous) != len(next) {
		return true
	}
	for i := range previous {
		if previous[i] != next[i] {
			return true
		}
	}
	return false
}

func (s MQTTWebhookService) ReportPathHealth(ctx context.Context, input MQTTPathHealthReportInput) error {
	input = normalizeMQTTPathHealthReportInput(input)
	if input.NetworkID == "" || input.DeviceID == "" {
		return ErrInvalidArgument
	}
	pathType := firstNonEmpty(input.PathType, input.ActivePath)
	if pathType == "" {
		return ErrInvalidArgument
	}
	if input.PeerNodeID != "" && !strings.HasPrefix(input.PeerNodeID, "node-") {
		return ErrInvalidArgument
	}
	items, err := s.Networks.ListNetworkDevices(ctx, input.NetworkID)
	if err != nil {
		return err
	}
	member := false
	for _, item := range items {
		if item.DeviceID == input.DeviceID && item.Enabled && item.Status == "active" {
			updated := item
			updated.ActivePath = pathType
			updated.PathObservedAt = input.SampledAtMs
			if updated.PathObservedAt <= 0 {
				updated.PathObservedAt = currentTime(s.Now).UnixMilli()
			}
			updated.RelayTransport = input.RelayTransport
			updated.RelayEndpoint = input.Endpoint
			updated.DerpNodeID = input.DerpNodeID
			updated.PeerNodeID = input.PeerNodeID
			updated.PathScore = input.PathScore
			updated.ObservedRttMs = input.ObservedRttMs
			updated.PacketLossPpm = input.PacketLossPpm
			updated.RelayMtu = input.RelayMtu
			updated.MaxFramePayload = input.MaxFramePayload
			updated.TicketExpiresAt = input.TicketExpiresAt
			updated.TicketRenewDue = input.TicketRenewDue
			updated.PathDowngrades = input.PathDowngrades
			updated.PathUpgrades = input.PathUpgrades
			updated.LastPathChange = input.LastPathChange
			updated.UpdatedAt = currentTime(s.Now).Unix()
			if err := s.Networks.SaveNetworkDevice(ctx, updated); err != nil {
				return err
			}
			member = true
			break
		}
	}
	if !member {
		return ErrNotFound
	}
	return forwardPathHealthReportToWire(ctx, input.NetworkID, input.DeviceID, input, pathType)
}

func normalizeMQTTPathHealthReportInput(input MQTTPathHealthReportInput) MQTTPathHealthReportInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.PeerNodeID = strings.TrimSpace(input.PeerNodeID)
	input.PathType = strings.TrimSpace(input.PathType)
	input.ActivePath = strings.TrimSpace(input.ActivePath)
	input.RelayTransport = strings.TrimSpace(input.RelayTransport)
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	input.DerpNodeID = strings.TrimSpace(input.DerpNodeID)
	input.TicketExpiresAt = strings.TrimSpace(input.TicketExpiresAt)
	input.LastPathChange = strings.TrimSpace(input.LastPathChange)
	if input.ObservedRttMs < 0 {
		input.ObservedRttMs = 0
	}
	if input.PacketLossPpm < 0 {
		input.PacketLossPpm = 0
	}
	if input.PathScore < 0 {
		input.PathScore = 0
	}
	if input.RelayMtu < 0 {
		input.RelayMtu = 0
	}
	if input.MaxFramePayload < 0 {
		input.MaxFramePayload = 0
	}
	if input.TicketExpiresInMs < 0 {
		input.TicketExpiresInMs = 0
	}
	if input.PathDowngrades < 0 {
		input.PathDowngrades = 0
	}
	if input.PathUpgrades < 0 {
		input.PathUpgrades = 0
	}
	if input.SampledAtMs < 0 {
		input.SampledAtMs = 0
	}
	return input
}

func forwardPathHealthReportToWire(ctx context.Context, networkID, deviceID string, payload MQTTPathHealthReportInput, pathType string) error {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SLAN_BIZ_WIRE_INTERNAL_URL")), "/")
	if baseURL == "" || !wirePathHealthForwardable(pathType) {
		return nil
	}
	body, err := json.Marshal(WirePeerPathHealthInput{
		PeerID: fmt.Sprintf("%s:%s", networkID, deviceID),
		Probes: []WirePathProbeInput{{
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
