package impl

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
)

func TestParseNetworkStateTopic(t *testing.T) {
	deviceID, networkID, ok := parseNetworkStateTopic(
		"slan/devices",
		"slan/devices/dev-1/networks/net-1/state",
	)
	if !ok || deviceID != "dev-1" || networkID != "net-1" {
		t.Fatalf("unexpected topic parse device=%s network=%s ok=%v", deviceID, networkID, ok)
	}
	if _, _, ok := parseNetworkStateTopic("slan/devices", "slan/devices/dev-1/state"); ok {
		t.Fatal("expected malformed topic to be rejected")
	}
}

func TestApplyMQTTNetworkStateUpsertsTrustedState(t *testing.T) {
	state := newNetworkTestState(t)
	state.cfg = configs.DefaultConfig()
	ctx := context.Background()
	now := time.Now().Unix()

	auth, err := (dbAuthService{state: state}).Register(dto.RegisterRequest{
		Email:    "u@example.com",
		Password: "Register-2026!",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, err := (dbDeviceService{state: state}).Register(auth.UserID, dto.RegisterDeviceRequest{
		Name:      "device",
		Platform:  "windows",
		MachineID: "machine-1",
		PublicKey: "public-key",
	})
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	network, err := (dbNetworkService{state: state}).Create(auth.UserID, dto.CreateNetworkRequest{
		Name:         "mqtt-net",
		CIDR:         "10.0.0.0/24",
		BindDeviceID: device.DeviceID,
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	if _, err := (dbNetworkService{state: state}).Activate(auth.UserID, network.NetworkID, dto.JoinNetworkRequest{
		DeviceID: device.DeviceID,
	}); err != nil {
		t.Fatalf("activate network: %v", err)
	}
	networkID := network.NetworkID

	payload, _ := json.Marshal(dto.DeviceNetworkStateRequest{
		DeviceID:         device.DeviceID,
		NetworkID:        networkID,
		ControlReachable: true,
		NetworkOnline:    true,
		TunnelUp:         true,
		LastProbeOK:      true,
		VirtualIP:        "10.0.0.2",
		ReportedAt:       now,
	})
	topic := "slan/devices/" + device.DeviceID + "/networks/" + networkID + "/state"
	if err := state.applyMQTTNetworkState(topic, payload); err != nil {
		t.Fatalf("apply mqtt state: %v", err)
	}
	got, err := state.pg.GetDeviceNetworkState(ctx, device.DeviceID, networkID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !got.ControlReachable || !got.NetworkOnline || !got.TunnelUp || !got.LastProbeOK || got.VirtualIP != "10.0.0.2" {
		t.Fatalf("unexpected state: %+v", got)
	}
}

func TestApplyMQTTNetworkStateKeepsHeartbeatWhenAttachmentDisabled(t *testing.T) {
	state := newNetworkTestState(t)
	state.cfg = configs.DefaultConfig()
	ctx := context.Background()
	now := time.Now().Unix()

	auth, err := (dbAuthService{state: state}).Register(dto.RegisterRequest{
		Email:    "u@example.com",
		Password: "Register-2026!",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, err := (dbDeviceService{state: state}).Register(auth.UserID, dto.RegisterDeviceRequest{
		Name:      "device",
		Platform:  "windows",
		MachineID: "machine-1",
		PublicKey: "public-key",
	})
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	network, err := (dbNetworkService{state: state}).Create(auth.UserID, dto.CreateNetworkRequest{
		Name:         "mqtt-net",
		CIDR:         "10.0.0.0/24",
		BindDeviceID: device.DeviceID,
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	activated, err := (dbNetworkService{state: state}).Activate(auth.UserID, network.NetworkID, dto.JoinNetworkRequest{
		DeviceID: device.DeviceID,
	})
	if err != nil {
		t.Fatalf("activate network: %v", err)
	}
	if _, err := (dbNetworkService{state: state}).UpdateAttachmentStatus(auth.UserID, network.NetworkID, activated.Attachment.AttachmentID, dto.UpdateAttachmentStatusRequest{
		Status: "disabled",
	}); err != nil {
		t.Fatalf("disable attachment: %v", err)
	}

	payload, _ := json.Marshal(dto.DeviceNetworkStateRequest{
		DeviceID:         device.DeviceID,
		NetworkID:        network.NetworkID,
		ControlReachable: true,
		NetworkOnline:    true,
		TunnelUp:         true,
		LastProbeOK:      true,
		VirtualIP:        "10.0.0.2",
		ReportedAt:       now,
	})
	topic := "slan/devices/" + device.DeviceID + "/networks/" + network.NetworkID + "/state"
	if err := state.applyMQTTNetworkState(topic, payload); err != nil {
		t.Fatalf("apply mqtt state: %v", err)
	}
	got, err := state.pg.GetDeviceNetworkState(ctx, device.DeviceID, network.NetworkID)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !got.ControlReachable || got.NetworkOnline || got.TunnelUp || got.LastProbeOK || got.VirtualIP != "" {
		t.Fatalf("expected disabled attachment to keep heartbeat but force network offline, got %+v", got)
	}
}

func TestApplyMQTTNetworkStateRejectsMismatchedPayload(t *testing.T) {
	state := newNetworkTestState(t)
	state.cfg = configs.DefaultConfig()
	payload, _ := json.Marshal(dto.DeviceNetworkStateRequest{
		DeviceID:         "other-device",
		NetworkID:        "net-1",
		ControlReachable: true,
	})
	if err := state.applyMQTTNetworkState("slan/devices/dev-1/networks/net-1/state", payload); err == nil {
		t.Fatal("expected mismatched device id to be rejected")
	}
}

func TestNormalizeDeviceNetworkRuntimeStateUsesAttachmentIP(t *testing.T) {
	networkOnline, tunnelUp, lastProbeOK, virtualIP := normalizeDeviceNetworkRuntimeState(
		dto.DeviceNetworkStateRequest{
			NetworkOnline: true,
			TunnelUp:      true,
			LastProbeOK:   true,
		},
		true,
		" 10.0.0.8 ",
	)
	if !networkOnline || !tunnelUp || !lastProbeOK || virtualIP != "10.0.0.8" {
		t.Fatalf("unexpected normalized state online=%v tunnel=%v probe=%v ip=%q", networkOnline, tunnelUp, lastProbeOK, virtualIP)
	}
}

func TestNormalizeDeviceNetworkRuntimeStateForcesDisabledAttachmentOffline(t *testing.T) {
	networkOnline, tunnelUp, lastProbeOK, virtualIP := normalizeDeviceNetworkRuntimeState(
		dto.DeviceNetworkStateRequest{
			NetworkOnline: true,
			TunnelUp:      true,
			LastProbeOK:   true,
			VirtualIP:     "10.0.0.9",
		},
		false,
		"10.0.0.8",
	)
	if networkOnline || tunnelUp || lastProbeOK || virtualIP != "" {
		t.Fatalf("expected disabled attachment offline state, got online=%v tunnel=%v probe=%v ip=%q", networkOnline, tunnelUp, lastProbeOK, virtualIP)
	}
}

func TestReportedDeviceNetworkStateTimeUsesRequestTimestamp(t *testing.T) {
	got := reportedDeviceNetworkStateTime(
		dto.DeviceNetworkStateRequest{ReportedAt: 123},
		time.Unix(456, 0),
	)
	if got != 123 {
		t.Fatalf("expected reported timestamp, got %d", got)
	}
}
