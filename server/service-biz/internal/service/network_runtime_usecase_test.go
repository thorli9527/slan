package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type networkRuntimeTestNetworks struct {
	networks            map[string]model.Network
	networkDevices      map[string][]model.NetworkDevice
	savedNetworkDevices []model.NetworkDevice
	securityGroups      map[string]model.SecurityGroup
	securityRules       map[string][]model.SecurityRule
}

func (s *networkRuntimeTestNetworks) GetNetwork(_ context.Context, networkID string) (model.Network, bool, error) {
	item, ok := s.networks[networkID]
	return item, ok, nil
}

func (s *networkRuntimeTestNetworks) ListNetworksByOwner(context.Context, string) ([]model.Network, error) {
	return nil, nil
}

func (s *networkRuntimeTestNetworks) ListNetworksByDevice(_ context.Context, deviceID string) ([]model.Network, error) {
	out := []model.Network{}
	for networkID, items := range s.networkDevices {
		for _, item := range items {
			if item.DeviceID == deviceID {
				if network, ok := s.networks[networkID]; ok {
					out = append(out, network)
				}
				break
			}
		}
	}
	return out, nil
}

func (s *networkRuntimeTestNetworks) SaveNetwork(context.Context, model.Network) error { return nil }
func (s *networkRuntimeTestNetworks) DeleteNetwork(context.Context, string) error      { return nil }

func (s *networkRuntimeTestNetworks) ListNetworkDevices(_ context.Context, networkID string) ([]model.NetworkDevice, error) {
	items := s.networkDevices[networkID]
	out := make([]model.NetworkDevice, len(items))
	copy(out, items)
	return out, nil
}

func (s *networkRuntimeTestNetworks) GetNetworkDevice(_ context.Context, networkID, deviceID string) (model.NetworkDevice, bool, error) {
	for _, item := range s.networkDevices[networkID] {
		if item.DeviceID == deviceID {
			return item, true, nil
		}
	}
	return model.NetworkDevice{}, false, nil
}

func (s *networkRuntimeTestNetworks) SaveNetworkDevice(_ context.Context, item model.NetworkDevice) error {
	s.savedNetworkDevices = append(s.savedNetworkDevices, item)
	return nil
}

func (s *networkRuntimeTestNetworks) GetNetworkVersion(_ context.Context, networkID string) (model.NetworkConfigVersion, bool, error) {
	return model.NetworkConfigVersion{}, false, nil
}

func (s *networkRuntimeTestNetworks) SaveNetworkVersion(context.Context, model.NetworkConfigVersion) error {
	return nil
}

func (s *networkRuntimeTestNetworks) DeleteNetworkDevice(context.Context, string, string) error {
	return nil
}

func (s *networkRuntimeTestNetworks) ListDeviceInvitesByUser(context.Context, string) ([]model.DeviceInvite, error) {
	return nil, nil
}

func (s *networkRuntimeTestNetworks) ListDeviceInvitesByNetwork(context.Context, string) ([]model.DeviceInvite, error) {
	return nil, nil
}

func (s *networkRuntimeTestNetworks) GetDeviceInvite(context.Context, string) (model.DeviceInvite, bool, error) {
	return model.DeviceInvite{}, false, nil
}

func (s *networkRuntimeTestNetworks) GetDeviceInviteByCode(context.Context, string) (model.DeviceInvite, bool, error) {
	return model.DeviceInvite{}, false, nil
}

func (s *networkRuntimeTestNetworks) SaveDeviceInvite(context.Context, model.DeviceInvite) error {
	return nil
}
func (s *networkRuntimeTestNetworks) ListDNSZones(context.Context, string) ([]model.DNSZone, error) {
	return nil, nil
}

func (s *networkRuntimeTestNetworks) GetDNSZone(context.Context, string) (model.DNSZone, bool, error) {
	return model.DNSZone{}, false, nil
}

func (s *networkRuntimeTestNetworks) SaveDNSZone(context.Context, model.DNSZone) error { return nil }
func (s *networkRuntimeTestNetworks) DeleteDNSZone(context.Context, string) error      { return nil }
func (s *networkRuntimeTestNetworks) ListDNSRecords(context.Context, string) ([]model.DNSRecord, error) {
	return nil, nil
}

func (s *networkRuntimeTestNetworks) GetDNSRecord(context.Context, string) (model.DNSRecord, bool, error) {
	return model.DNSRecord{}, false, nil
}

func (s *networkRuntimeTestNetworks) SaveDNSRecord(context.Context, model.DNSRecord) error {
	return nil
}
func (s *networkRuntimeTestNetworks) DeleteDNSRecord(context.Context, string) error { return nil }
func (s *networkRuntimeTestNetworks) ListSecurityGroups(_ context.Context, networkID string) ([]model.SecurityGroup, error) {
	out := []model.SecurityGroup{}
	for _, item := range s.securityGroups {
		if item.NetworkID == networkID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *networkRuntimeTestNetworks) GetSecurityGroup(_ context.Context, securityGroupID string) (model.SecurityGroup, bool, error) {
	item, ok := s.securityGroups[securityGroupID]
	return item, ok, nil
}

func (s *networkRuntimeTestNetworks) SaveSecurityGroup(context.Context, model.SecurityGroup) error {
	return nil
}

func (s *networkRuntimeTestNetworks) DeleteSecurityGroup(context.Context, string) error { return nil }

func (s *networkRuntimeTestNetworks) ListSecurityRules(_ context.Context, securityGroupID string) ([]model.SecurityRule, error) {
	items := s.securityRules[securityGroupID]
	out := make([]model.SecurityRule, len(items))
	copy(out, items)
	return out, nil
}

func (s *networkRuntimeTestNetworks) GetSecurityRule(_ context.Context, ruleID string) (model.SecurityRule, bool, error) {
	for _, items := range s.securityRules {
		for _, item := range items {
			if item.RuleID == ruleID {
				return item, true, nil
			}
		}
	}
	return model.SecurityRule{}, false, nil
}

func (s *networkRuntimeTestNetworks) SaveSecurityRule(context.Context, model.SecurityRule) error {
	return nil
}

func (s *networkRuntimeTestNetworks) DeleteSecurityRule(context.Context, string) error { return nil }

type networkRuntimeTestDevices struct {
	devices                 map[string]model.Device
	groupAssignmentsByUser  map[string][]model.DeviceGroupAssignment
	lastGroupAssignmentUser string
}

func (s *networkRuntimeTestDevices) ListDevicesByOwner(_ context.Context, ownerID string) ([]model.Device, error) {
	out := []model.Device{}
	for _, item := range s.devices {
		if item.OwnerID == ownerID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *networkRuntimeTestDevices) GetDevice(_ context.Context, deviceID string) (model.Device, bool, error) {
	item, ok := s.devices[deviceID]
	return item, ok, nil
}

func (s *networkRuntimeTestDevices) SaveDevice(context.Context, model.Device) error { return nil }
func (s *networkRuntimeTestDevices) DeleteDevice(context.Context, string) error     { return nil }
func (s *networkRuntimeTestDevices) NewDeviceVirtualIPID() string {
	return "vip00000000000000000000000000000001"
}

func (s *networkRuntimeTestDevices) GetDeviceLoginDevice(context.Context, string) (model.DeviceLoginDevice, bool, error) {
	return model.DeviceLoginDevice{}, false, nil
}

func (s *networkRuntimeTestDevices) SaveDeviceLoginDevice(context.Context, model.DeviceLoginDevice) error {
	return nil
}

func (s *networkRuntimeTestDevices) GetDeviceSessionByAccessToken(context.Context, string) (model.DeviceSession, bool, error) {
	return model.DeviceSession{}, false, nil
}

func (s *networkRuntimeTestDevices) GetDeviceSessionByRefreshToken(context.Context, string) (model.DeviceSession, bool, error) {
	return model.DeviceSession{}, false, nil
}

func (s *networkRuntimeTestDevices) ListDeviceSessionsByDeviceID(context.Context, string) ([]model.DeviceSession, error) {
	return nil, nil
}

func (s *networkRuntimeTestDevices) SaveDeviceSession(context.Context, model.DeviceSession) error {
	return nil
}

func (s *networkRuntimeTestDevices) DeleteDeviceSessionByAccessToken(context.Context, string) error {
	return nil
}

func (s *networkRuntimeTestDevices) ListDeviceBootstrapKeys(context.Context, string) ([]model.DeviceBootstrapKey, error) {
	return nil, nil
}

func (s *networkRuntimeTestDevices) GetDeviceBootstrapKey(context.Context, string) (model.DeviceBootstrapKey, bool, error) {
	return model.DeviceBootstrapKey{}, false, nil
}

func (s *networkRuntimeTestDevices) GetDeviceBootstrapKeyByToken(context.Context, string) (model.DeviceBootstrapKey, bool, error) {
	return model.DeviceBootstrapKey{}, false, nil
}

func (s *networkRuntimeTestDevices) SaveDeviceBootstrapKey(context.Context, model.DeviceBootstrapKey) error {
	return nil
}

func (s *networkRuntimeTestDevices) ListDeviceGroups(context.Context, string) ([]model.DeviceGroup, error) {
	return nil, nil
}

func (s *networkRuntimeTestDevices) GetDeviceGroup(context.Context, string) (model.DeviceGroup, bool, error) {
	return model.DeviceGroup{}, false, nil
}

func (s *networkRuntimeTestDevices) SaveDeviceGroup(context.Context, model.DeviceGroup) error {
	return nil
}
func (s *networkRuntimeTestDevices) DeleteDeviceGroup(context.Context, string) error { return nil }
func (s *networkRuntimeTestDevices) SetDeviceGroups(context.Context, model.DeviceGroupAssignment) error {
	return nil
}
func (s *networkRuntimeTestDevices) ListDeviceGroupAssignments(_ context.Context, userID string) ([]model.DeviceGroupAssignment, error) {
	s.lastGroupAssignmentUser = userID
	if s.groupAssignmentsByUser != nil {
		return append([]model.DeviceGroupAssignment(nil), s.groupAssignmentsByUser[userID]...), nil
	}
	return []model.DeviceGroupAssignment{
		{UserID: "user-1", DeviceID: "src", GroupIDs: []string{"group-src"}},
		{UserID: "user-1", DeviceID: "dst", GroupIDs: []string{"group-dst"}},
	}, nil
}

func TestBuildNetworkConfigUsesNetworkOwnerGroupsForSharedDevice(t *testing.T) {
	devices := &networkRuntimeTestDevices{
		devices: map[string]model.Device{
			"shared": {DeviceID: "shared", OwnerID: "device-owner", VirtualIP: "10.0.0.1"},
			"peer":   {DeviceID: "peer", VirtualIP: "10.0.0.12"},
		},
		groupAssignmentsByUser: map[string][]model.DeviceGroupAssignment{
			"network-owner": {
				{UserID: "network-owner", DeviceID: "shared", GroupIDs: []string{"group-dev"}},
				{UserID: "network-owner", DeviceID: "peer", GroupIDs: []string{"group-dev"}},
			},
		},
	}
	networks := &networkRuntimeTestNetworks{
		networkDevices: map[string][]model.NetworkDevice{
			"net-shared": {
				{NetworkID: "net-shared", DeviceID: "shared", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
				{NetworkID: "net-shared", DeviceID: "peer", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
		securityGroups: map[string]model.SecurityGroup{},
		securityRules:  map[string][]model.SecurityRule{},
	}

	view, err := buildNetworkConfigView(
		context.Background(),
		nil,
		devices,
		networks,
		model.Network{NetworkID: "net-shared", OwnerID: "network-owner", CIDR: "10.0.0.0/24"},
		devices.devices["shared"],
	)
	if err != nil {
		t.Fatalf("build shared network config: %v", err)
	}
	if devices.lastGroupAssignmentUser != "network-owner" {
		t.Fatalf("loaded assignments for %q, want network owner", devices.lastGroupAssignmentUser)
	}
	if got := view.DeviceGroupsByDevice["shared"]; len(got) != 1 || got[0] != "group-dev" {
		t.Fatalf("shared device groups = %#v", got)
	}
	if got := view.DeviceGroupsByDevice["peer"]; len(got) != 1 || got[0] != "group-dev" {
		t.Fatalf("peer groups = %#v", got)
	}
}

type networkRuntimeTestOps struct {
	relayNodes []model.RelayNode
}

func (s *networkRuntimeTestOps) ListRelayNodes(context.Context) ([]model.RelayNode, error) {
	out := make([]model.RelayNode, len(s.relayNodes))
	copy(out, s.relayNodes)
	return out, nil
}

func (s *networkRuntimeTestOps) GetRelayNode(_ context.Context, nodeID string) (model.RelayNode, bool, error) {
	for _, item := range s.relayNodes {
		if item.NodeID == nodeID {
			return item, true, nil
		}
	}
	return model.RelayNode{}, false, nil
}

func (s *networkRuntimeTestOps) SaveRelayNode(context.Context, model.RelayNode) error { return nil }
func (s *networkRuntimeTestOps) DeleteRelayNode(context.Context, string) error        { return nil }
func (s *networkRuntimeTestOps) ListPunchNodes(context.Context) ([]model.PunchNode, error) {
	return nil, nil
}

func (s *networkRuntimeTestOps) GetPunchNode(context.Context, string) (model.PunchNode, bool, error) {
	return model.PunchNode{}, false, nil
}

func (s *networkRuntimeTestOps) SavePunchNode(context.Context, model.PunchNode) error { return nil }
func (s *networkRuntimeTestOps) DeletePunchNode(context.Context, string) error        { return nil }

var _ repository.NetworkRepository = (*networkRuntimeTestNetworks)(nil)
var _ repository.DeviceRepository = (*networkRuntimeTestDevices)(nil)
var _ repository.OpsNodeRepository = (*networkRuntimeTestOps)(nil)

func TestIssueRelayTicketRejectsBroadIngressDeny(t *testing.T) {
	service := newNetworkRuntimeTestService([]model.SecurityRule{{
		RuleID:          "sgr-1",
		SecurityGroupID: "sg-1",
		Direction:       "ingress",
		Protocol:        "all",
		PortRange:       "all",
		PeerType:        "device_group",
		PeerValue:       "group-src",
		Action:          "deny",
		Priority:        5,
		Enabled:         true,
	}})

	_, err := service.IssueRelayTicket(context.Background(), IssueRelayTicketInput{
		NetworkID: "net-1",
		SrcNodeID: "node-src",
		DstNodeID: "node-dst",
		Reason:    "test",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestIssueRelayTicketAllowsPortScopedDeny(t *testing.T) {
	t.Setenv("SLAN_RELAY_TICKET_SECRET", "relay-secret-current")
	t.Setenv("SLAN_WIRE_TICKET_SECRET", "relay-secret-legacy")
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "relay-secret-ring,relay-secret-old")
	service := newNetworkRuntimeTestService([]model.SecurityRule{{
		RuleID:          "sgr-1",
		SecurityGroupID: "sg-1",
		Direction:       "egress",
		Protocol:        "tcp",
		PortRange:       "443",
		CIDR:            "peer:device:dst",
		Action:          "deny",
		Priority:        5,
		Enabled:         true,
	}})

	view, err := service.IssueRelayTicket(context.Background(), IssueRelayTicketInput{
		NetworkID: "net-1",
		SrcNodeID: "node-src",
		DstNodeID: "node-dst",
		Reason:    "test",
	})
	if err != nil {
		t.Fatalf("IssueRelayTicket returned error: %v", err)
	}
	if view.TicketID == "" || view.RelayURL == "" {
		t.Fatalf("IssueRelayTicket returned incomplete relay ticket: %+v", view)
	}
	payload := strings.Join([]string{
		view.TicketID,
		view.NetworkID,
		view.SessionID,
		view.SrcNodeID,
		view.DstNodeID,
		view.ExpiresAt,
	}, "|")
	mac := hmac.New(sha256.New, []byte("relay-secret-current"))
	_, _ = mac.Write([]byte(payload))
	want := hex.EncodeToString(mac.Sum(nil))
	if view.Signature != want {
		t.Fatalf("IssueRelayTicket returned wrong signature: got=%q want=%q", view.Signature, want)
	}
}

func TestIssueRelayTicketReusesStableSessionForSameNodePair(t *testing.T) {
	service := newNetworkRuntimeTestService(nil)

	forward, err := service.IssueRelayTicket(context.Background(), IssueRelayTicketInput{
		NetworkID: "net-1",
		SrcNodeID: "node-src",
		DstNodeID: "node-dst",
		Reason:    "forward",
	})
	if err != nil {
		t.Fatalf("forward IssueRelayTicket returned error: %v", err)
	}

	reverse, err := service.IssueRelayTicket(context.Background(), IssueRelayTicketInput{
		NetworkID: "net-1",
		SrcNodeID: "node-dst",
		DstNodeID: "node-src",
		Reason:    "reverse",
	})
	if err != nil {
		t.Fatalf("reverse IssueRelayTicket returned error: %v", err)
	}

	if forward.SessionID == "" || reverse.SessionID == "" {
		t.Fatalf("expected non-empty session IDs, got forward=%q reverse=%q", forward.SessionID, reverse.SessionID)
	}
	if forward.SessionID != reverse.SessionID {
		t.Fatalf("expected same relay session for same node pair, got forward=%q reverse=%q", forward.SessionID, reverse.SessionID)
	}
	if forward.RelayURL != reverse.RelayURL {
		t.Fatalf("expected stable relay target, got forward=%q reverse=%q", forward.RelayURL, reverse.RelayURL)
	}
}

func TestCreatePunchConnectSessionRejectsMissingPeerMembership(t *testing.T) {
	service := newNetworkRuntimeTestService(nil)
	_, err := service.CreatePunchConnectSession(context.Background(), CreatePunchConnectSessionInput{
		NetworkID:       "net-1",
		RequesterNodeID: "node-src",
		PeerNodeID:      "node-missing",
		TTLSeconds:      30,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCreatePunchConnectSessionRejectsMissingRequesterMembership(t *testing.T) {
	service := newNetworkRuntimeTestService(nil)
	_, err := service.CreatePunchConnectSession(context.Background(), CreatePunchConnectSessionInput{
		NetworkID:       "net-1",
		RequesterNodeID: "node-missing",
		PeerNodeID:      "node-dst",
		TTLSeconds:      30,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func newNetworkRuntimeTestService(rules []model.SecurityRule) NetworkRuntimeService {
	now := time.Unix(1700000000, 0)
	return NetworkRuntimeService{
		Devices: &networkRuntimeTestDevices{
			devices: map[string]model.Device{
				"src": {DeviceID: "src", OwnerID: "user-1", VirtualIP: "10.0.0.1", Alias: "mac-src", Name: "Mac"},
				"dst": {DeviceID: "dst", OwnerID: "user-1", VirtualIP: "10.0.0.2", Alias: "ios-dst", Name: "iPhone"},
			},
		},
		Networks: &networkRuntimeTestNetworks{
			networks: map[string]model.Network{
				"net-1": {
					NetworkID: "net-1",
					Name:      "Default",
					CIDR:      "10.0.0.0/24",
					Status:    "active",
				},
			},
			networkDevices: map[string][]model.NetworkDevice{
				"net-1": {
					{NetworkID: "net-1", DeviceID: "src", Enabled: true, MemberStatus: model.NetworkMemberStatusActive, PresenceStatus: model.DevicePresenceStatusOffline},
					{NetworkID: "net-1", DeviceID: "dst", Enabled: true, MemberStatus: model.NetworkMemberStatusActive, PresenceStatus: model.DevicePresenceStatusOffline},
				},
			},
			securityGroups: map[string]model.SecurityGroup{
				"sg-1": {SecurityGroupID: "sg-1", NetworkID: "net-1", Name: "Default"},
			},
			securityRules: map[string][]model.SecurityRule{
				"sg-1": rules,
			},
		},
		Ops: &networkRuntimeTestOps{
			relayNodes: []model.RelayNode{{
				NodeID:    "relay-1",
				Name:      "Relay",
				Region:    "dev",
				Endpoint:  "127.0.0.1:29110",
				Transport: "relay_udp",
				Status:    "active",
				Health:    "healthy",
				Priority:  1,
				UpdatedAt: now.Unix(),
			}},
		},
		NewSessID: func(scope string) string { return scope + "-1" },
		Now:       func() time.Time { return now },
	}
}
