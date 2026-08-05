package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestObserveDeviceLocationPersistsResolvedLocation(t *testing.T) {
	devices := &networkRuntimeTestDevices{devices: map[string]model.Device{
		"device-1": {DeviceID: "device-1", UpdatedAt: 1},
	}}
	service := NetworkRuntimeService{
		Devices: devices,
		LocateIP: func(ip string) (DeviceLocation, bool) {
			if ip != "203.0.113.10" {
				t.Fatalf("lookup IP = %q", ip)
			}
			return DeviceLocation{CountryCode: "hk", CityCode: "1819729"}, true
		},
		Now: func() time.Time { return time.Unix(1_700_000_000, 0) },
	}

	if err := service.ObserveDeviceLocation(context.Background(), "device-1", "203.0.113.10"); err != nil {
		t.Fatalf("ObserveDeviceLocation returned error: %v", err)
	}
	device := devices.devices["device-1"]
	if device.PublicIP != "203.0.113.10" || device.CountryCode != "HK" || device.CityCode != "1819729" {
		t.Fatalf("persisted device = %+v", device)
	}
}

func TestIssueRelayTicketUsesClientSelectedEndpoint(t *testing.T) {
	service := newNetworkRuntimeTestService(nil)
	service.Ops = &networkRuntimeTestOps{relayNodes: []model.RelayNode{
		{NodeID: "relay-a", Region: "relay-a", Endpoint: "203.0.113.10:29110", Transport: "relay_udp", Priority: 1, Status: "active", Health: "healthy", UpdatedAt: 1_700_000_000},
		{NodeID: "relay-b", Region: "relay-b", Endpoint: "203.0.113.11:29110", Transport: "relay_udp", Priority: 2, Status: "active", Health: "healthy", UpdatedAt: 1_700_000_000},
	}}

	view, err := service.IssueRelayTicket(context.Background(), IssueRelayTicketInput{
		NetworkID:       "net-1",
		SrcNodeID:       "node-src",
		DstNodeID:       "node-dst",
		RelayEndpointID: "relay-b",
	})
	if err != nil {
		t.Fatalf("IssueRelayTicket returned error: %v", err)
	}
	if view.RelayURL != "udp://203.0.113.11:29110" || view.DERPClusterID != "relay-b" {
		t.Fatalf("relay ticket = %+v", view)
	}
}

func TestCreatePunchConnectSessionUsesClientSelectedNode(t *testing.T) {
	service := newNetworkRuntimeTestService(nil)
	service.Ops = &networkRuntimeTestOps{punchNodes: []model.PunchNode{
		{NodeID: "punch-a", Endpoint: "203.0.113.20:29130", Priority: 1, Status: "active", Health: "healthy"},
		{NodeID: "punch-b", Endpoint: "203.0.113.21:29130", Priority: 2, Status: "active", Health: "healthy"},
	}}

	view, err := service.CreatePunchConnectSession(context.Background(), CreatePunchConnectSessionInput{
		NetworkID:       "net-1",
		RequesterNodeID: "node-src",
		PeerNodeID:      "node-dst",
		PunchNodeID:     "punch-b",
	})
	if err != nil {
		t.Fatalf("CreatePunchConnectSession returned error: %v", err)
	}
	if view.PunchNodeID != "punch-b" || view.Requester == nil || view.Requester.Address != "203.0.113.21:29130" {
		t.Fatalf("punch session = %+v", view)
	}
}
