package biz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func mustMarshalRawMessage(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal raw message: %v", err)
	}
	return data
}

func TestRegisterUserAndDeviceJoinDefaultNetwork(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	user := auth.User
	if network.NetworkID != "default-"+user.UserID || network.OwnerUserID != user.UserID {
		t.Fatalf("expected user to own default network, got %+v", network)
	}
	if network.Code != "default" {
		t.Fatalf("expected default network code, got %+v", network)
	}

	device, deviceMember, err := store.RegisterDevice(user.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "work mac", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if device.GlobalIP != "10.0.0.2" || device.GlobalName != "mac-1.staticlss.com" {
		t.Fatalf("unexpected global device identity: %+v", device)
	}
	if device.PrefixLen != ipamSubnetPrefix || device.GlobalCIDR != ipamGlobalCIDR || device.SubnetCIDR != "10.0.0.0/20" || device.SubnetPrefixLen != ipamSubnetPrefix {
		t.Fatalf("expected registered device to include subnet data, got %+v", device)
	}
	listed, err := store.GetDevice(device.DeviceID)
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if listed.SubnetID == "" || listed.SubnetCIDR != device.SubnetCIDR || listed.GlobalCIDR != ipamGlobalCIDR {
		t.Fatalf("expected listed device to include subnet data, got %+v", listed)
	}
	if deviceMember.NetworkID != network.NetworkID || !deviceMember.Enabled {
		t.Fatalf("expected device to join default network enabled, got %+v", deviceMember)
	}
}

func TestBindDeviceSessionTransfersSamePhysicalDeviceBetweenUsers(t *testing.T) {
	store := NewStore()
	aliceAuth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	bobAuth, _, err := store.RegisterUser("bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	aliceDevice, aliceSession, _, err := store.BindDeviceSession(aliceAuth.Session.Token, "same-client", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	if err != nil {
		t.Fatalf("bind alice device session: %v", err)
	}
	bobDevice, bobSession, _, err := store.BindDeviceSession(bobAuth.Session.Token, "same-client", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	if err != nil {
		t.Fatalf("bind bob device session: %v", err)
	}
	if aliceDevice.DeviceID != bobDevice.DeviceID {
		t.Fatalf("expected same physical device id after user switch, got alice=%s bob=%s", aliceDevice.DeviceID, bobDevice.DeviceID)
	}
	if aliceDevice.OwnerID != aliceAuth.User.UserID || bobDevice.OwnerID != bobAuth.User.UserID {
		t.Fatalf("unexpected device owners: alice=%+v bob=%+v", aliceDevice, bobDevice)
	}
	if aliceSession.DeviceID != aliceDevice.DeviceID || bobSession.DeviceID != bobDevice.DeviceID {
		t.Fatalf("session device ids should match returned devices: alice=%+v bob=%+v", aliceSession, bobSession)
	}
	devices := store.ListDevices("")
	if len(devices) != 1 || devices[0].OwnerID != bobAuth.User.UserID {
		t.Fatalf("expected one transferred device owned by bob, got %+v", devices)
	}
	if _, _, _, err := store.RenewDeviceSession(aliceSession.DeviceToken, true, 1, 1); err != errUnauthorized {
		t.Fatalf("expected old user device session to be revoked, got %v", err)
	}
}

func TestDeviceSessionResponseIncludesRuntimeEndpoints(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("runtime@example.com", "secret", "Runtime")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	if _, err := store.UpsertRelayNode(OpsRelayNode{
		Name:       "UDP Relay",
		Region:     "ap-east",
		Transport:  "relay_udp",
		PublicAddr: "47.245.40.231:29110",
		Status:     "active",
		Health:     "healthy",
		Priority:   1,
	}); err != nil {
		t.Fatalf("upsert udp relay: %v", err)
	}
	if _, err := store.UpsertRelayNode(OpsRelayNode{
		Name:       "TCP Relay",
		Region:     "ap-east",
		Transport:  "derp_tcp_tls_443",
		PublicAddr: "47.245.40.231:29120",
		Status:     "active",
		Health:     "healthy",
		Priority:   2,
	}); err != nil {
		t.Fatalf("upsert tcp relay: %v", err)
	}
	if _, err := store.UpsertPunchNode(OpsPunchNode{
		Name:          "Punch",
		Region:        "ap-east",
		PublicUDPIP:   "47.245.40.231",
		PublicUDPPort: 29130,
		Status:        "active",
		Health:        "healthy",
		Priority:      1,
	}); err != nil {
		t.Fatalf("upsert punch: %v", err)
	}
	device, session, configs, err := store.BindDeviceSession(auth.Session.Token, "runtime-mac", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("bind device session: %v", err)
	}
	runtime := RuntimeService{
		store: store,
		mqtt: MQTTConfig{
			Enabled:              true,
			PublicBrokerURL:      "mqtt://47.245.40.231:1883",
			UsernamePrefix:       "slan",
			PasswordSecret:       "secret",
			TopicPrefix:          "slan",
			CredentialTTLSeconds: 3600,
		},
	}
	response := runtime.DeviceSessionResponse(device, session, configs, time.Unix(1000, 0))
	if response.MQTT == nil || response.MQTT.BrokerURL != "mqtt://47.245.40.231:1883" {
		t.Fatalf("expected mqtt broker in session response, got %+v", response.MQTT)
	}
	if response.RuntimeEndpoints.MQTT == nil || response.RuntimeEndpoints.MQTT.BrokerURL != response.MQTT.BrokerURL {
		t.Fatalf("expected runtime mqtt to mirror top-level mqtt, got %+v", response.RuntimeEndpoints.MQTT)
	}
	if len(response.RuntimeEndpoints.PunchNodes) != 1 || response.RuntimeEndpoints.PunchNodes[0].Address != "47.245.40.231:29130" {
		t.Fatalf("expected punch endpoint by ip:port, got %+v", response.RuntimeEndpoints.PunchNodes)
	}
	transportByAddress := map[string]string{}
	for _, candidate := range response.RuntimeEndpoints.RelayCandidates {
		transportByAddress[candidate.Address] = candidate.Transport
	}
	if transportByAddress["47.245.40.231:29110"] != "udp" || transportByAddress["47.245.40.231:29120"] != "derp_tcp_tls_443" {
		t.Fatalf("unexpected relay runtime endpoints: %+v", response.RuntimeEndpoints.RelayCandidates)
	}
	if len(response.RuntimeEndpoints.Networks) != 1 || response.RuntimeEndpoints.Networks[0].NetworkID == "" || len(response.RuntimeEndpoints.Networks[0].RelayCandidates) < 2 {
		t.Fatalf("expected per-network relay endpoints, got %+v", response.RuntimeEndpoints.Networks)
	}
}

func TestDeviceSessionBindRejectsDuplicateDeviceIDWithDifferentPublicKey(t *testing.T) {
	store := NewStore()
	aliceAuth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	bobAuth, _, err := store.RegisterUser("bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	if _, _, _, err := store.BindDeviceSession(aliceAuth.Session.Token, "same-client", "Mac", "macos", "macOS", "15.0", "", "pub-a"); err != nil {
		t.Fatalf("bind alice device session: %v", err)
	}
	if _, _, _, err := store.BindDeviceSession(bobAuth.Session.Token, "same-client", "Mac", "macos", "macOS", "15.0", "", "pub-b"); err != errConflict {
		t.Fatalf("expected duplicate device id with different public key to conflict, got %v", err)
	}
	device, err := store.GetDevice("same-client")
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.OwnerID != aliceAuth.User.UserID || device.PublicKey != "pub-a" {
		t.Fatalf("expected original device identity to remain unchanged, got %+v", device)
	}
}

func TestRegisterDeviceRejectsCrossUserDeviceIDReuse(t *testing.T) {
	store := NewStore()
	aliceAuth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	bobAuth, _, err := store.RegisterUser("bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	if _, _, err := store.RegisterDevice(aliceAuth.User.UserID, "same-client", "Mac", "macos", "macOS", "15.0", "", "pub-a"); err != nil {
		t.Fatalf("register alice device: %v", err)
	}
	if _, _, err := store.RegisterDevice(bobAuth.User.UserID, "same-client", "Mac", "macos", "macOS", "15.0", "", "pub-b"); err != errConflict {
		t.Fatalf("expected cross-user duplicate device registration to conflict, got %v", err)
	}
	device, err := store.GetDevice("same-client")
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.OwnerID != aliceAuth.User.UserID {
		t.Fatalf("expected owner to remain alice, got %+v", device)
	}
}

func TestUpdateCustomerProfilePersistsOpsFields(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	customer, err := store.UpdateCustomerProfile(CustomerProfile{
		CustomerID: auth.User.UserID,
		Email:      "alice.ops@example.com",
		Name:       "Alice Ops",
		Country:    "中国",
		Province:   "广东",
		City:       "深圳",
		IPRegion:   "华南",
		Status:     "limited",
	})
	if err != nil {
		t.Fatalf("update customer profile: %v", err)
	}
	if customer.Email != "alice.ops@example.com" || customer.Name != "Alice Ops" || customer.Country != "中国" || customer.IPRegion != "华南" || customer.Status != "limited" {
		t.Fatalf("unexpected updated customer: %+v", customer)
	}
	customers := store.ListCustomers()
	if len(customers) != 1 || customers[0].City != "深圳" || customers[0].Status != "limited" {
		t.Fatalf("expected persisted ops customer fields, got %+v", customers)
	}
	if _, err := store.LoginUser("alice.ops@example.com", "secret"); err != nil {
		t.Fatalf("limited customer should still be able to login: %v", err)
	}
	if _, err := store.UpdateCustomerProfile(CustomerProfile{CustomerID: auth.User.UserID, Email: "alice.ops@example.com", Name: "Alice Ops", Status: "disabled"}); err != nil {
		t.Fatalf("disable customer: %v", err)
	}
	if _, err := store.LoginUser("alice.ops@example.com", "secret"); err != errBadRequest {
		t.Fatalf("disabled customer login should fail with bad request, got %v", err)
	}
}

func TestUpdateCustomerProfileDisabledRevokesSessionsAndNetwork(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, deviceSession, _, err := store.BindDeviceSession(auth.Session.Token, "mac-customer-disable-1", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("bind device session: %v", err)
	}
	if !store.HasActiveNetworkDevice(network.NetworkID, device.DeviceID) {
		t.Fatalf("expected active network membership before disable")
	}
	if _, err := store.UpdateCustomerProfile(CustomerProfile{
		CustomerID: auth.User.UserID,
		Email:      auth.User.Email,
		Name:       auth.User.Name,
		Status:     "disabled",
	}); err != nil {
		t.Fatalf("disable customer: %v", err)
	}
	if _, err := store.AuthByToken(auth.Session.Token); err != errNotFound {
		t.Fatalf("expected disabled customer user session removed, got %v", err)
	}
	if _, _, _, err := store.RenewDeviceSession(deviceSession.DeviceToken, true, 1, 1); err != errUnauthorized {
		t.Fatalf("expected disabled customer device session revoked, got %v", err)
	}
	if store.HasActiveNetworkDevice(network.NetworkID, device.DeviceID) {
		t.Fatalf("expected disabled customer device inactive in network")
	}
	memberships := store.ListNetworkDevicesForUser(auth.User.UserID)
	if len(memberships) != 1 || memberships[0].Enabled || memberships[0].Status != "disabled" {
		t.Fatalf("expected disabled customer membership, got %+v", memberships)
	}
	disabledDevice, err := store.GetDevice(device.DeviceID)
	if err != nil {
		t.Fatalf("get disabled device: %v", err)
	}
	if disabledDevice.Status != "disabled" {
		t.Fatalf("expected disabled device status, got %+v", disabledDevice)
	}
}

func TestOpsDeviceDisableUpdatesRuntimeAndMemberships(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-ops-1", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	disabled := false
	view, err := store.UpdateOpsDevice(device.DeviceID, "", "", &disabled)
	if err != nil {
		t.Fatalf("ops disable device: %v", err)
	}
	if view.DeviceEnabled {
		t.Fatalf("expected ops device view disabled, got %+v", view)
	}
	if store.HasActiveNetworkDevice(network.NetworkID, device.DeviceID) {
		t.Fatalf("expected disabled ops device to be inactive in network")
	}
	memberships := store.ListNetworkDevicesForDevice(device.DeviceID)
	if len(memberships) != 1 || memberships[0].Enabled || memberships[0].Status != "disabled" {
		t.Fatalf("expected disabled membership, got %+v", memberships)
	}
}

func TestRelayNodePublicAddressUniqueAndHealthReadOnly(t *testing.T) {
	store := NewStore()
	node, err := store.UpsertRelayNode(OpsRelayNode{
		Name:             "Relay A",
		Region:           "hk",
		Transport:        "relay_udp",
		PublicAddr:       "udp://relay.example.com:29110",
		MaxBandwidthMbps: 100,
		MonthlyTrafficGB: 1000,
		MaxSessions:      1000,
		Status:           "active",
	})
	if err != nil {
		t.Fatalf("create relay node: %v", err)
	}
	if _, err := store.UpsertRelayNode(OpsRelayNode{
		Name:             "Relay B",
		Region:           "hk",
		Transport:        "relay_udp",
		PublicAddr:       "udp://relay.example.com:29110",
		MaxBandwidthMbps: 100,
		MonthlyTrafficGB: 1000,
		MaxSessions:      1000,
		Status:           "active",
	}); err != errConflict {
		t.Fatalf("expected duplicate public address conflict, got %v", err)
	}
	store.mu.Lock()
	existing := store.relayNodes[node.NodeID]
	existing.Health = "warning"
	existing.ActiveSessions = 42
	existing.UsedTrafficGB = 7
	store.relayNodes[node.NodeID] = existing
	store.mu.Unlock()
	updated, err := store.UpsertRelayNode(OpsRelayNode{
		NodeID:           node.NodeID,
		Name:             "Relay A Updated",
		Region:           "hk",
		Transport:        "relay_udp",
		PublicAddr:       "udp://relay-a.example.com:29110",
		MaxBandwidthMbps: 200,
		MonthlyTrafficGB: 2000,
		MaxSessions:      2000,
		Status:           "maintenance",
		Health:           "healthy",
	})
	if err != nil {
		t.Fatalf("update relay node: %v", err)
	}
	if updated.Health != "warning" || updated.ActiveSessions != 42 || updated.UsedTrafficGB != 7 {
		t.Fatalf("expected runtime fields to be preserved, got %+v", updated)
	}
}

func TestNetworkConfigReturnsPeersACLAndDNS(t *testing.T) {
	store := NewStore()
	auth, network, _ := store.RegisterUser("alice@example.com", "secret", "Alice")
	user := auth.User
	deviceA, _, _ := store.RegisterDevice(user.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	deviceB, _, _ := store.RegisterDevice(user.UserID, "ios-1", "iPhone", "ios", "iOS", "18.0", "", "pub-b")
	if _, _, err := store.RenewDevice(deviceB.DeviceID, user.UserID, true, 0, 0); err != nil {
		t.Fatalf("mark peer active: %v", err)
	}
	groups := store.ListSecurityGroups(network.NetworkID)
	if len(groups) == 0 {
		t.Fatal("expected default security group")
	}
	rule, err := store.AddSecurityGroupRule(groups[0].SecurityGroupID, "ingress", "allow", "tcp", "device", deviceA.DeviceID, "ssh", 100, 22, 22, true)
	if err != nil {
		t.Fatalf("add acl: %v", err)
	}
	zones := store.ListDNSZones(network.NetworkID)
	if len(zones) == 0 {
		t.Fatal("expected default dns zone")
	}
	record, err := store.AddDNSRecord(network.NetworkID, zones[0].ZoneID, "phone", "A", deviceB.DeviceID, "", "", "443", 60)
	if err != nil {
		t.Fatalf("add dns: %v", err)
	}

	config, err := store.NetworkConfig(network.NetworkID, deviceA.DeviceID)
	if err != nil {
		t.Fatalf("network config: %v", err)
	}
	if config.GlobalIP != deviceA.GlobalIP || len(config.Peers) != 1 || config.Peers[0].DeviceID != deviceB.DeviceID {
		t.Fatalf("unexpected network config peers: %+v", config)
	}
	if config.Peers[0].SubnetID == "" || config.Peers[0].SubnetCIDR != "10.0.0.0/20" || config.Peers[0].SubnetPrefixLen != ipamSubnetPrefix {
		t.Fatalf("expected peer device to include subnet data, got %+v", config.Peers[0])
	}
	if !config.Peers[0].RelayAllowed {
		t.Fatalf("expected peer device to allow relay sessions, got %+v", config.Peers[0])
	}
	if len(config.Rules) != 1 || config.Rules[0].RuleID != rule.RuleID {
		t.Fatalf("unexpected acl: %+v", config.Rules)
	}
	if len(config.DNSRecords) != 1 || config.DNSRecords[0].RecordID != record.RecordID {
		t.Fatalf("unexpected dns: %+v", config.DNSRecords)
	}
}

func TestNetworkConfigIncludesEnabledPeerBeforeRuntimeReport(t *testing.T) {
	store := NewStore()
	auth, network, _ := store.RegisterUser("alice@example.com", "secret", "Alice")
	deviceA, _, _ := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	deviceB, _, _ := store.RegisterDevice(auth.User.UserID, "ios-1", "iPhone", "ios", "iOS", "18.0", "", "pub-b")

	config, err := store.NetworkConfig(network.NetworkID, deviceA.DeviceID)
	if err != nil {
		t.Fatalf("network config before runtime report: %v", err)
	}
	if len(config.Peers) != 1 || config.Peers[0].DeviceID != deviceB.DeviceID {
		t.Fatalf("expected enabled peer before runtime report, got %+v", config.Peers)
	}
}

func TestNetworkConfigIncludesPeerEndpointReport(t *testing.T) {
	store := NewStore()
	auth, network, _ := store.RegisterUser("endpoint@example.com", "secret", "Endpoint")
	deviceA, _, _ := store.RegisterDevice(auth.User.UserID, "mac-endpoint", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	deviceB, _, _ := store.RegisterDevice(auth.User.UserID, "android-endpoint", "Android", "android", "Android", "15", "", "pub-b")
	otherNetwork, _, _, err := store.CreateNetwork(auth.User.UserID, "Other", "other", "default")
	if err != nil {
		t.Fatalf("create other network: %v", err)
	}
	if _, err := store.AddNetworkDevice(otherNetwork.NetworkID, deviceB.DeviceID, auth.User.UserID, "", true); err != nil {
		t.Fatalf("add other network device: %v", err)
	}
	if changed, err := store.ReportDeviceEndpoint(otherNetwork.NetworkID, deviceB.DeviceID, []DeviceEndpoint{{
		Type:      "direct_udp",
		Address:   "198.51.100.20:40123",
		UpdatedAt: time.Now().Unix(),
	}}); err != nil || !changed {
		t.Fatalf("report endpoint in other network: %v", err)
	}

	if changed, err := store.ReportDeviceEndpoint(network.NetworkID, deviceB.DeviceID, []DeviceEndpoint{{
		Type:      "direct_udp",
		Address:   "192.0.2.10:40123",
		UpdatedAt: time.Now().Unix(),
	}}); err != nil || !changed {
		t.Fatalf("report endpoint: %v", err)
	}
	config, err := store.NetworkConfig(network.NetworkID, deviceA.DeviceID)
	if err != nil {
		t.Fatalf("network config: %v", err)
	}
	if len(config.Peers) != 1 || len(config.Peers[0].Endpoints) != 1 || config.Peers[0].Endpoints[0].Address != "192.0.2.10:40123" {
		t.Fatalf("expected peer endpoint in config, got %+v", config.Peers)
	}
}

func TestNetworkConfigNormalizesCIDRGlobalIPs(t *testing.T) {
	store := NewStore()
	auth, network, _ := store.RegisterUser("alice@example.com", "secret", "Alice")
	deviceA, _, _ := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	deviceB, _, _ := store.RegisterDevice(auth.User.UserID, "ios-1", "iPhone", "ios", "iOS", "18.0", "", "pub-b")
	store.ReportDeviceRuntime(deviceB.DeviceID, true, 10, 20)

	store.mu.Lock()
	currentA := store.devices[deviceA.DeviceID]
	currentA.GlobalIP = "10.0.0.9/32"
	store.devices[deviceA.DeviceID] = currentA
	currentB := store.devices[deviceB.DeviceID]
	currentB.GlobalIP = "10.0.0.10/32"
	store.devices[deviceB.DeviceID] = currentB
	store.mu.Unlock()

	config, err := store.NetworkConfig(network.NetworkID, deviceA.DeviceID)
	if err != nil {
		t.Fatalf("network config: %v", err)
	}
	if config.GlobalIP != "10.0.0.9" {
		t.Fatalf("expected host global ip, got %q", config.GlobalIP)
	}
	if len(config.Peers) != 1 || config.Peers[0].GlobalIP != "10.0.0.10" {
		t.Fatalf("expected host peer ip, got %+v", config.Peers)
	}
}

func TestNetworkConfigsForDeviceReturnsAllEnabledMemberships(t *testing.T) {
	store := NewStore()
	auth, defaultNetwork, _ := store.RegisterUser("alice@example.com", "secret", "Alice")
	user := auth.User
	deviceA, _, _ := store.RegisterDevice(user.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	networkB, _, _, err := store.CreateNetwork(user.UserID, "开发网络", "dev", "dev")
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	if _, err := store.AddNetworkDevice(networkB.NetworkID, deviceA.DeviceID, user.UserID, "", true); err != nil {
		t.Fatalf("add device to second network: %v", err)
	}
	configs, err := store.NetworkConfigsForDevice(deviceA.DeviceID)
	if err != nil {
		t.Fatalf("device network configs: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected two network configs, got %+v", configs)
	}
	ids := map[string]bool{}
	for _, config := range configs {
		ids[config.NetworkID] = true
		if config.DeviceID != deviceA.DeviceID || config.GlobalIP != deviceA.GlobalIP {
			t.Fatalf("unexpected config device identity: %+v", config)
		}
	}
	if !ids[defaultNetwork.NetworkID] || !ids[networkB.NetworkID] {
		t.Fatalf("missing expected networks: %+v", ids)
	}
}

func TestNetworkDeviceEnableStateControlsMQTTMembership(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if !store.HasActiveNetworkDevice(network.NetworkID, device.DeviceID) {
		t.Fatalf("expected default network device to be active")
	}
	disabled := false
	updated, err := store.UpdateNetworkDevice(network.NetworkID, device.DeviceID, "disabled mac", &disabled)
	if err != nil {
		t.Fatalf("disable network device: %v", err)
	}
	if updated.Enabled || updated.Status != "disabled" || store.HasActiveNetworkDevice(network.NetworkID, device.DeviceID) {
		t.Fatalf("expected disabled network membership, got %+v", updated)
	}
	enabled := true
	updated, err = store.UpdateNetworkDevice(network.NetworkID, device.DeviceID, "enabled mac", &enabled)
	if err != nil {
		t.Fatalf("enable network device: %v", err)
	}
	if !updated.Enabled || updated.Status != "active" || !store.HasActiveNetworkDevice(network.NetworkID, device.DeviceID) {
		t.Fatalf("expected enabled network membership, got %+v", updated)
	}
}

func TestMQTTTopicAccessSeparatesQoS0UpstreamFromQoS2DownstreamTopics(t *testing.T) {
	cfg := MQTTConfig{Enabled: true, TopicPrefix: "slan"}
	deviceID := "mac-1"
	if !mqttAllowTopicAccess(cfg, "device", deviceID, "slan/devices/mac-1/heartbeat", false) {
		t.Fatalf("device heartbeat topic should be publishable")
	}
	if !mqttAllowTopicAccess(cfg, "device", deviceID, "slan/devices/mac-1/runtime", false) {
		t.Fatalf("device runtime topic should be publishable")
	}
	if !mqttAllowTopicAccess(cfg, "device", deviceID, "slan/devices/mac-1/control/down", true) {
		t.Fatalf("device control down topic should be subscribable")
	}
	if !mqttAllowTopicAccess(cfg, "server", "", "slan/devices/mac-1/control/down", false) {
		t.Fatalf("server should publish control down topics")
	}
	if !mqttAllowTopicAccess(cfg, "server", "", "slan/networks/net-1/broadcast", false) {
		t.Fatalf("server should publish network broadcast topics")
	}
	if !mqttAllowTopicAccess(cfg, "server", "", "slan/devices/#", true) {
		t.Fatalf("server should subscribe device upstream wildcard topic")
	}
	if mqttAllowTopicAccess(cfg, "device", deviceID, "slan/devices/ios-1/heartbeat", false) {
		t.Fatalf("device must not publish another device heartbeat")
	}
}

func TestGlobalIPPoolPreGeneratesAndRefills(t *testing.T) {
	store := NewStore()
	subnets := store.ListIPAMSubnets()
	if len(subnets) != 1 || subnets[0].CIDRBlock != "10.0.0.0/20" || subnets[0].GeneratedCapacity != 4093 {
		t.Fatalf("unexpected initial ip pool: %+v", subnets)
	}

	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	for i := 0; i < 3095; i++ {
		device, _, err := store.RegisterDevice(auth.User.UserID, "device-refill-"+stringID(i), "Device", "linux", "Linux", "1.0", "", "")
		if err != nil {
			t.Fatalf("register device %d: %v", i, err)
		}
		if i == 0 && device.GlobalIP != "10.0.0.2" {
			t.Fatalf("expected first assigned ip 10.0.0.2, got %s", device.GlobalIP)
		}
	}
	subnets = store.ListIPAMSubnets()
	if len(subnets) != 2 || subnets[1].CIDRBlock != "10.0.16.0/20" {
		t.Fatalf("expected second subnet after low watermark refill, got %+v", subnets)
	}
}

func TestRegisterDeviceAssignsUniqueGlobalIPsAcrossUsers(t *testing.T) {
	store := NewStore()
	aliceAuth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	bobAuth, _, err := store.RegisterUser("bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}

	seen := map[string]string{}
	for i := 0; i < 80; i++ {
		userID := aliceAuth.User.UserID
		owner := "alice"
		if i%2 == 1 {
			userID = bobAuth.User.UserID
			owner = "bob"
		}
		deviceID := owner + "-device-" + stringID(i)
		device, _, err := store.RegisterDevice(userID, deviceID, "Device", "linux", "Linux", "1.0", "", "pub-"+deviceID)
		if err != nil {
			t.Fatalf("register device %s: %v", deviceID, err)
		}
		if device.GlobalIP == "" {
			t.Fatalf("device %s got empty global IP", deviceID)
		}
		if existing := seen[device.GlobalIP]; existing != "" {
			t.Fatalf("duplicate global IP assigned: ip=%s devices=%s,%s", device.GlobalIP, existing, deviceID)
		}
		seen[device.GlobalIP] = deviceID
	}
}

func TestNetworkConfigRepairsDuplicateDeviceGlobalIPs(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	deviceA, _, err := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	if err != nil {
		t.Fatalf("register device a: %v", err)
	}
	deviceB, _, err := store.RegisterDevice(auth.User.UserID, "linux-1", "Linux", "linux", "Linux", "6.0", "", "pub-b")
	if err != nil {
		t.Fatalf("register device b: %v", err)
	}
	if _, _, err := store.RenewDevice(deviceB.DeviceID, auth.User.UserID, true, 0, 0); err != nil {
		t.Fatalf("mark peer active: %v", err)
	}

	store.mu.Lock()
	deviceB.GlobalIP = deviceA.GlobalIP
	store.devices[deviceB.DeviceID] = deviceB
	store.mu.Unlock()

	config, err := store.NetworkConfig(network.NetworkID, deviceA.DeviceID)
	if err != nil {
		t.Fatalf("network config: %v", err)
	}
	if len(config.Peers) != 1 {
		t.Fatalf("expected one peer, got %+v", config.Peers)
	}
	if config.GlobalIP == config.Peers[0].GlobalIP {
		t.Fatalf("expected duplicate IP to be repaired, got self=%s peer=%s", config.GlobalIP, config.Peers[0].GlobalIP)
	}
	repaired := store.ListDevices(auth.User.UserID)
	seen := map[string]string{}
	for _, device := range repaired {
		if existing := seen[device.GlobalIP]; existing != "" {
			t.Fatalf("duplicate global IP after repair: ip=%s devices=%s,%s", device.GlobalIP, existing, device.DeviceID)
		}
		seen[device.GlobalIP] = device.DeviceID
	}
}

func TestMQTTControlAckIsRecorded(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-ack-1", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	now := timeNow().Unix()
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-1", "network_config_changed", "enableNetwork", map[string]string{"networkId": "net-1"}, now, now+500); err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	server := &Server{
		store: store,
		mqtt:  MQTTConfig{TopicPrefix: "slan"},
	}
	body, err := json.Marshal(map[string]any{
		"taskId":        "downstream-enableNetwork-delivery-1",
		"deliveryId":    "delivery-1",
		"action":        "enableNetwork",
		"status":        "succeeded",
		"processedAtMs": int64(12345),
	})
	if err != nil {
		t.Fatalf("marshal ack: %v", err)
	}

	if err := server.handleMQTTDevicePublish(context.Background(), "slan/devices/"+device.DeviceID+"/control/ack", body); err != nil {
		t.Fatalf("handle control ack: %v", err)
	}

	delivery, err := store.GetMQTTControlDelivery("delivery-1")
	if err != nil {
		t.Fatalf("get recorded delivery: %v", err)
	}
	if delivery.DeviceID != device.DeviceID || delivery.Status != "succeeded" || delivery.Action != "enableNetwork" || delivery.ProcessedAtMs != 12345 {
		t.Fatalf("unexpected recorded delivery: %+v", delivery)
	}
	if !hasAuditEvent(store.ListAuditEvents(), "client_network.control_ack", "succeeded", "delivery-1") {
		t.Fatalf("expected network control ack audit event, got %+v", store.ListAuditEvents())
	}
}

func TestMQTTControlAckRejectsWrongDeviceTopic(t *testing.T) {
	store := NewStore()
	server := &Server{
		store: store,
		mqtt:  MQTTConfig{TopicPrefix: "slan"},
	}
	body := []byte(`{"taskId":"task-1","deliveryId":"delivery-1","action":"enableNetwork","status":"succeeded"}`)

	if err := server.handleMQTTDevicePublish(context.Background(), "slan/devices/missing-device/control/ack", body); err != errNotFound {
		t.Fatalf("expected unknown device ack to be rejected, got %v", err)
	}
}

func TestMQTTControlAckRejectsUnknownDelivery(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-ack-unknown", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}

	if _, err := store.RecordMQTTControlAck(device.DeviceID, "unknown-delivery", "task-1", "enableNetwork", "succeeded", "", 1, 100); err != errNotFound {
		t.Fatalf("expected unknown delivery ack to be rejected, got %v", err)
	}
}

func TestMQTTControlAckHandlerIgnoresExpiredDelivery(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-ack-expired-handler", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "expired-handler", "network_config_changed", "enableNetwork", map[string]string{"networkId": "net-1"}, 100, 120); err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	if expired := store.ExpireMQTTControlDeliveries(120); expired != 1 {
		t.Fatalf("expected delivery to expire, got %d", expired)
	}
	server := &Server{store: store, mqtt: MQTTConfig{TopicPrefix: "slan"}}
	body := []byte(`{"taskId":"task-expired","deliveryId":"expired-handler","action":"enableNetwork","status":"succeeded","processedAtMs":123}`)

	if err := server.handleMQTTDevicePublish(context.Background(), "slan/devices/"+device.DeviceID+"/control/ack", body); err != nil {
		t.Fatalf("expected expired mqtt ack to be consumed without handler error, got %v", err)
	}
	delivery, err := store.GetMQTTControlDeliveryForDevice(device.DeviceID, "expired-handler")
	if err != nil {
		t.Fatalf("get expired delivery: %v", err)
	}
	if delivery.Status != "expired" || delivery.AckedAt != 0 {
		t.Fatalf("expected expired delivery to remain unacked, got %+v", delivery)
	}
}

func TestMQTTRuntimeNetworkStateChangeWritesAuditEvent(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-runtime-audit", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	server := &Server{store: store, mqtt: MQTTConfig{TopicPrefix: "slan"}}
	body := []byte(`{"networkEnabled":true,"rxBytesTotal":10,"txBytesTotal":20}`)

	if err := server.handleMQTTDevicePublish(context.Background(), "slan/devices/"+device.DeviceID+"/runtime-state", body); err != nil {
		t.Fatalf("handle runtime state: %v", err)
	}
	if !hasAuditEvent(store.ListAuditEvents(), "client_network.runtime_state_changed", "succeeded", device.DeviceID) {
		t.Fatalf("expected runtime state audit event, got %+v", store.ListAuditEvents())
	}
}

func TestMQTTPathHealthReportIsAcceptedForNetworkMember(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-path-health", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	server := &Server{store: store, mqtt: MQTTConfig{TopicPrefix: "slan"}}
	body, err := json.Marshal(controlEnvelopeRaw{
		Type: "path_health_report",
		Payload: mustMarshalRawMessage(t, map[string]any{
			"networkId":       network.NetworkID,
			"pathType":        "relay_udp",
			"relayTransport":  "relay_udp",
			"endpoint":        "203.0.113.10:29110",
			"derpNodeId":      "relay-hk-1",
			"observedRttMs":   31,
			"packetLossPpm":   0,
			"pathScore":       31,
			"relayMtu":        1280,
			"maxFramePayload": 1200,
			"sampledAtMs":     int64(123456),
		}),
	})
	if err != nil {
		t.Fatalf("encode path health envelope: %v", err)
	}

	if err := server.handleMQTTDevicePublish(context.Background(), "slan/devices/"+device.DeviceID+"/control/up", body); err != nil {
		t.Fatalf("handle path health report: %v", err)
	}
}

func TestMQTTPathHealthReportForwardsToWireWhenConfigured(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-path-health-forward", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	var received wirePathHealthRequest
	wireServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/peers/path-health" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		if got := r.Header.Get("X-Slan-Internal-Token"); got != "wire-token" {
			t.Fatalf("expected internal token, got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode forwarded request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer wireServer.Close()
	t.Setenv("SLAN_BIZ_WIRE_INTERNAL_URL", wireServer.URL)
	t.Setenv("SLAN_INTERNAL_WIRE_TOKEN", "wire-token")

	server := &Server{store: store, mqtt: MQTTConfig{TopicPrefix: "slan"}}
	body, err := json.Marshal(controlEnvelopeRaw{
		Type: "path_health_report",
		Payload: mustMarshalRawMessage(t, map[string]any{
			"networkId":     network.NetworkID,
			"pathType":      "derp_tcp_tls_443",
			"observedRttMs": 42,
			"packetLossPpm": 1000,
			"pathScore":     1042,
			"relayMtu":      1280,
			"sampledAtMs":   int64(987654321000),
		}),
	})
	if err != nil {
		t.Fatalf("encode path health envelope: %v", err)
	}

	if err := server.handleMQTTDevicePublish(context.Background(), "slan/devices/"+device.DeviceID+"/control/up", body); err != nil {
		t.Fatalf("handle path health report: %v", err)
	}
	if received.PeerID != wirePeerID(network.NetworkID, device.DeviceID) {
		t.Fatalf("unexpected forwarded peer id %q", received.PeerID)
	}
	if len(received.Probes) != 1 {
		t.Fatalf("expected one forwarded probe, got %+v", received.Probes)
	}
	probe := received.Probes[0]
	if probe.Path != "derp_tcp_tls_443" || !probe.Reachable || probe.RTTMs != 42 || probe.LossPPM != 1000 || probe.MTU != 1280 || probe.ObservedAt != 987654321000 {
		t.Fatalf("unexpected forwarded probe %+v", probe)
	}
}

func TestMQTTPathHealthReportRejectsWrongNetwork(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-path-health-wrong-net", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	server := &Server{store: store, mqtt: MQTTConfig{TopicPrefix: "slan"}}
	body, err := json.Marshal(controlEnvelopeRaw{
		Type: "path_health_report",
		Payload: mustMarshalRawMessage(t, map[string]any{
			"networkId": "missing-network",
			"pathType":  "relay_tcp",
		}),
	})
	if err != nil {
		t.Fatalf("encode path health envelope: %v", err)
	}

	if err := server.handleMQTTDevicePublish(context.Background(), "slan/devices/"+device.DeviceID+"/control/up", body); !errors.Is(err, errNotFound) {
		t.Fatalf("expected wrong network to be rejected, got %v", err)
	}
}

func TestMQTTPathHealthReportRequiresPath(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-path-health-no-path", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	server := &Server{store: store, mqtt: MQTTConfig{TopicPrefix: "slan"}}
	body, err := json.Marshal(controlEnvelopeRaw{
		Type: "path_health_report",
		Payload: mustMarshalRawMessage(t, map[string]any{
			"networkId": network.NetworkID,
			"endpoint":  "203.0.113.10:29110",
		}),
	})
	if err != nil {
		t.Fatalf("encode path health envelope: %v", err)
	}

	if err := server.handleMQTTDevicePublish(context.Background(), "slan/devices/"+device.DeviceID+"/control/up", body); err == nil || !strings.Contains(err.Error(), "pathType or activePath") {
		t.Fatalf("expected missing path to be rejected, got %v", err)
	}
}

func TestMQTTControlAckSameDeliveryIDIsScopedByDevice(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	deviceA, _, err := store.RegisterDevice(auth.User.UserID, "mac-ack-a", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	if err != nil {
		t.Fatalf("register device a: %v", err)
	}
	deviceB, _, err := store.RegisterDevice(auth.User.UserID, "mac-ack-b", "Mac", "macos", "macOS", "15.0", "", "pub-b")
	if err != nil {
		t.Fatalf("register device b: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(deviceA.DeviceID, "same-delivery", "network_config_changed", "enableNetwork", map[string]string{"networkId": "net-a"}, 100, 500); err != nil {
		t.Fatalf("prepare delivery a: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(deviceB.DeviceID, "same-delivery", "network_config_changed", "disableNetwork", map[string]string{"networkId": "net-b"}, 100, 500); err != nil {
		t.Fatalf("prepare delivery b: %v", err)
	}
	if _, err := store.RecordMQTTControlAck(deviceA.DeviceID, "same-delivery", "task-a", "enableNetwork", "succeeded", "", 1, 100); err != nil {
		t.Fatalf("record ack a: %v", err)
	}
	if _, err := store.RecordMQTTControlAck(deviceB.DeviceID, "same-delivery", "task-b", "disableNetwork", "failed", "boom", 2, 101); err != nil {
		t.Fatalf("record ack b: %v", err)
	}

	deliveryA, err := store.GetMQTTControlDeliveryForDevice(deviceA.DeviceID, "same-delivery")
	if err != nil {
		t.Fatalf("get ack a: %v", err)
	}
	deliveryB, err := store.GetMQTTControlDeliveryForDevice(deviceB.DeviceID, "same-delivery")
	if err != nil {
		t.Fatalf("get ack b: %v", err)
	}
	if deliveryA.TaskID != "task-a" || deliveryA.Status != "succeeded" {
		t.Fatalf("unexpected ack a: %+v", deliveryA)
	}
	if deliveryB.TaskID != "task-b" || deliveryB.Status != "failed" || deliveryB.Error != "boom" {
		t.Fatalf("unexpected ack b: %+v", deliveryB)
	}
}

func TestMQTTControlDeliveryTracksPublishAttemptsAndAck(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-delivery-1", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	prepared, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-track", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 100, 400)
	if err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	if prepared.Status != "pending" || prepared.MessageType != "network_config_changed" || prepared.ExpiresAt != 400 {
		t.Fatalf("unexpected prepared delivery: %+v", prepared)
	}
	if string(prepared.Payload) != `{"networkId":"net-1"}` {
		t.Fatalf("expected delivery payload to be stored, got %s", string(prepared.Payload))
	}
	failed, err := store.RecordMQTTControlPublishResult(device.DeviceID, "delivery-track", false, "broker unavailable", 110)
	if err != nil {
		t.Fatalf("record failed publish: %v", err)
	}
	if failed.Status != "retrying" || failed.AttemptCount != 1 || failed.Error != "broker unavailable" {
		t.Fatalf("unexpected failed publish delivery: %+v", failed)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-track", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 120, 400); err != nil {
		t.Fatalf("prepare retry delivery: %v", err)
	}
	published, err := store.RecordMQTTControlPublishResult(device.DeviceID, "delivery-track", true, "", 130)
	if err != nil {
		t.Fatalf("record published: %v", err)
	}
	if published.Status != "published" || published.AttemptCount != 2 || published.PublishedAt != 130 {
		t.Fatalf("unexpected published delivery: %+v", published)
	}
	acked, err := store.RecordMQTTControlAck(device.DeviceID, "delivery-track", "task-track", "client-update", "succeeded", "", 12345, 140)
	if err != nil {
		t.Fatalf("record ack: %v", err)
	}
	if acked.Status != "succeeded" || acked.AttemptCount != 2 || acked.MessageType != "network_config_changed" || acked.PublishedAt != 130 || acked.Action != "update" {
		t.Fatalf("expected ack to preserve publish metadata, got %+v", acked)
	}
}

func TestMQTTControlDeliveryRetrySelectionAndExpiry(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-delivery-retry", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-due", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 100, 500); err != nil {
		t.Fatalf("prepare due delivery: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-later", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 100, 500); err != nil {
		t.Fatalf("prepare later delivery: %v", err)
	}
	if _, err := store.RecordMQTTControlPublishResult(device.DeviceID, "delivery-later", false, "temporary", 110); err != nil {
		t.Fatalf("record later failure: %v", err)
	}
	due := store.ListRetryableMQTTControlDeliveries(110, 10)
	if len(due) != 1 || due[0].DeliveryID != "delivery-due" {
		t.Fatalf("expected only first attempt due at 110, got %+v", due)
	}
	due = store.ListRetryableMQTTControlDeliveries(111, 10)
	if len(due) != 2 {
		t.Fatalf("expected retrying delivery due after backoff, got %+v", due)
	}
	if expired := store.ExpireMQTTControlDeliveries(500); expired != 2 {
		t.Fatalf("expected two expired deliveries, got %d", expired)
	}
	due = store.ListRetryableMQTTControlDeliveries(501, 10)
	if len(due) != 0 {
		t.Fatalf("expected expired deliveries not retryable, got %+v", due)
	}
	expiredDelivery, err := store.GetMQTTControlDeliveryForDevice(device.DeviceID, "delivery-due")
	if err != nil {
		t.Fatalf("get expired delivery: %v", err)
	}
	if expiredDelivery.Status != "expired" {
		t.Fatalf("expected delivery expired, got %+v", expiredDelivery)
	}
}

func TestMQTTControlDeliveryGetShowsLogicalExpiryBeforeWorkerRuns(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-logical-expiry", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-logical-expired", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 100, 120); err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}

	delivery, err := store.GetMQTTControlDeliveryForDevice(device.DeviceID, "delivery-logical-expired")
	if err != nil {
		t.Fatalf("get delivery: %v", err)
	}
	if delivery.Status != "expired" {
		t.Fatalf("expected logical expired status before worker runs, got %+v", delivery)
	}
}

func TestMQTTControlPublishResultAfterTTLExpiresDelivery(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-publish-expired", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-publish-expired", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 100, 120); err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	delivery, err := store.RecordMQTTControlPublishResult(device.DeviceID, "delivery-publish-expired", true, "", 120)
	if err != nil {
		t.Fatalf("record publish result: %v", err)
	}
	if delivery.Status != "expired" || delivery.PublishedAt != 0 {
		t.Fatalf("expected publish result at ttl to expire delivery without publishedAt, got %+v", delivery)
	}
}

func TestMQTTControlDeliveryPublishedWithoutAckIsRetryable(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-published-retry", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-published", "device_user_login_succeeded", "login", map[string]string{"userId": auth.User.UserID}, 100, 500); err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	if _, err := store.RecordMQTTControlPublishResult(device.DeviceID, "delivery-published", true, "", 110); err != nil {
		t.Fatalf("record published: %v", err)
	}
	if due := store.ListRetryableMQTTControlDeliveries(111, 10); len(due) != 1 || due[0].DeliveryID != "delivery-published" {
		t.Fatalf("expected published delivery without ack to be retryable, got %+v", due)
	}
	if _, err := store.RecordMQTTControlAck(device.DeviceID, "delivery-published", "task-login", "login", "succeeded", "", 123, 120); err != nil {
		t.Fatalf("record ack: %v", err)
	}
	if due := store.ListRetryableMQTTControlDeliveries(200, 10); len(due) != 0 {
		t.Fatalf("expected acked delivery not retryable, got %+v", due)
	}
}

func TestMQTTControlAckRejectsExpiredDeliveryAndDuplicateAckIsIdempotent(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-expired-ack", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-expired", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 100, 120); err != nil {
		t.Fatalf("prepare expired delivery: %v", err)
	}
	if _, err := store.RecordMQTTControlAck(device.DeviceID, "delivery-expired", "task-expired", "update", "succeeded", "", 123, 120); err != errConflict {
		t.Fatalf("expected expired ack to be rejected, got %v", err)
	}

	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-once", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 100, 500); err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	first, err := store.RecordMQTTControlAck(device.DeviceID, "delivery-once", "task-once", "update", "succeeded", "", 123, 130)
	if err != nil {
		t.Fatalf("record first ack: %v", err)
	}
	second, err := store.RecordMQTTControlAck(device.DeviceID, "delivery-once", "task-late", "update", "failed", "late duplicate", 456, 140)
	if err != nil {
		t.Fatalf("record duplicate ack: %v", err)
	}
	if second.Status != first.Status || second.TaskID != first.TaskID || second.AckedAt != first.AckedAt || second.Error != first.Error {
		t.Fatalf("expected duplicate ack to preserve first ack, first=%+v second=%+v", first, second)
	}
}

func TestMQTTControlDeliveryRetryClaimPreventsDuplicateWorkerPublish(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-claim-retry", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-claim", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 100, 500); err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	due := store.ListRetryableMQTTControlDeliveries(110, 10)
	if len(due) != 1 {
		t.Fatalf("expected retryable delivery, got %+v", due)
	}
	claimed, err := store.ClaimMQTTControlDeliveryRetry(device.DeviceID, "delivery-claim", due[0].UpdatedAt, 110)
	if err != nil || !claimed {
		t.Fatalf("expected first worker to claim delivery, claimed=%v err=%v", claimed, err)
	}
	claimed, err = store.ClaimMQTTControlDeliveryRetry(device.DeviceID, "delivery-claim", due[0].UpdatedAt, 110)
	if err != nil {
		t.Fatalf("second claim failed unexpectedly: %v", err)
	}
	if claimed {
		t.Fatalf("expected second worker with stale updatedAt not to claim delivery")
	}
}

func TestMQTTControlDeliveryRetryClaimExpiresStaleDelivery(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-claim-expired", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	prepared, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-claim-expired", "network_config_changed", "update", map[string]string{"networkId": "net-1"}, 100, 120)
	if err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	claimed, err := store.ClaimMQTTControlDeliveryRetry(device.DeviceID, "delivery-claim-expired", prepared.UpdatedAt, 120)
	if err != nil {
		t.Fatalf("claim expired delivery: %v", err)
	}
	if claimed {
		t.Fatalf("expected expired delivery not to be claimed")
	}
	delivery, err := store.GetMQTTControlDeliveryForDevice(device.DeviceID, "delivery-claim-expired")
	if err != nil {
		t.Fatalf("get delivery: %v", err)
	}
	if delivery.Status != "expired" {
		t.Fatalf("expected claim to persist expired status, got %+v", delivery)
	}
}

func TestMQTTControlAckPersistsAcrossStoreRestart(t *testing.T) {
	path := t.TempDir() + "/mqtt-control-deliveries.json"
	store := NewStore()
	store.controlDeliveryStorePath = path
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-ack-persist", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.PrepareMQTTControlDelivery(device.DeviceID, "delivery-persist", "network_config_changed", "enableNetwork", map[string]string{"networkId": "net-1"}, 100, 500); err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	if _, err := store.RecordMQTTControlAck(device.DeviceID, "delivery-persist", "task-persist", "enableNetwork", "succeeded", "", 123, 456); err != nil {
		t.Fatalf("record ack: %v", err)
	}

	restarted := NewStore()
	restarted.controlDeliveryStorePath = path
	if err := restarted.loadControlDeliveriesLocked(); err != nil {
		t.Fatalf("load persisted deliveries: %v", err)
	}
	delivery, err := restarted.GetMQTTControlDeliveryForDevice(device.DeviceID, "delivery-persist")
	if err != nil {
		t.Fatalf("get persisted ack: %v", err)
	}
	if delivery.TaskID != "task-persist" || delivery.Status != "succeeded" || delivery.AckedAt != 456 {
		t.Fatalf("unexpected persisted ack: %+v", delivery)
	}
}

func TestControlEnvelopeIncludesReliabilityMetadata(t *testing.T) {
	envelope := newControlEnvelope(MQTTConfig{ControlMessageTTLSeconds: 120}, "network_config_changed", "msg-1", map[string]string{"networkId": "net-1"})

	if envelope.Type != "network_config_changed" || envelope.MessageID != "msg-1" {
		t.Fatalf("unexpected envelope identity: %+v", envelope)
	}
	if envelope.SchemaVersion != mqttControlSchemaVersion {
		t.Fatalf("unexpected schema version: %+v", envelope)
	}
	if envelope.CreatedAt <= 0 || envelope.ExpiresAt-envelope.CreatedAt != 120 {
		t.Fatalf("expected ttl metadata, got createdAt=%d expiresAt=%d", envelope.CreatedAt, envelope.ExpiresAt)
	}
}

func TestNetworkConfigChangedPayloadIncludesSubnetData(t *testing.T) {
	store := NewStore()
	auth, network, _ := store.RegisterUser("alice@example.com", "secret", "Alice")
	device, _, _ := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	server := &Server{store: store}

	payload := server.networkChangePayloadForDevice(networkChangePayload{
		NetworkID:     network.NetworkID,
		ConfigVersion: 1,
		Reason:        "test",
		ChangedAt:     100,
	}, device.DeviceID)

	if payload.VirtualIP != device.GlobalIP {
		t.Fatalf("expected host virtual IP %q, got %q", device.GlobalIP, payload.VirtualIP)
	}
	if payload.PrefixLen != ipamSubnetPrefix || payload.SubnetCIDR == "" || payload.SubnetPrefixLen != ipamSubnetPrefix || payload.GlobalCIDR != ipamGlobalCIDR {
		t.Fatalf("expected subnet data, got %+v", payload)
	}
}

func TestDeviceInviteIsSingleUseAndExpiresInThirtyMinutes(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	_, _, err = store.RegisterDevice(auth.User.UserID, "ios-1", "iPhone", "ios", "iOS", "18.0", "Alice Phone", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.AddNetworkDevice(network.NetworkID, "ios-1", auth.User.UserID, "", true); err != errConflict {
		t.Fatalf("expected duplicate default network add conflict, got %v", err)
	}
	otherAuth, otherNetwork, err := store.RegisterUser("bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register other user: %v", err)
	}
	if _, err := store.AddNetworkDevice(otherNetwork.NetworkID, "ios-1", otherAuth.User.UserID, "", true); err != errBadRequest {
		t.Fatalf("expected invisible device add to fail, got %v", err)
	}
	visibleInvite, err := store.CreateDeviceInvite(otherAuth.User.UserID, 3600)
	if err != nil {
		t.Fatalf("create visible invite: %v", err)
	}
	if visibleInvite.ExpiresAt-visibleInvite.CreatedAt != int64(deviceInviteTTL.Seconds()) {
		t.Fatalf("expected 30 minute ttl, got invite %+v", visibleInvite)
	}
	if len(visibleInvite.InviteCode) != 32 {
		t.Fatalf("expected 32 char invite code, got %q", visibleInvite.InviteCode)
	}
	grant, accepted, err := store.AcceptDeviceInvite(visibleInvite.InviteCode, "ios-1", auth.User.UserID)
	if err != nil {
		t.Fatalf("accept visible invite: %v", err)
	}
	if grant.DeviceID != "ios-1" || grant.UserID != otherAuth.User.UserID || accepted.Status != "accepted" {
		t.Fatalf("unexpected accepted invite grant=%+v invite=%+v", grant, accepted)
	}
	if _, _, err := store.AcceptDeviceInvite(visibleInvite.InviteCode, "ios-1", auth.User.UserID); err != errNotFound {
		t.Fatalf("expected invite single-use not found, got %v", err)
	}
	visible := store.ListVisibleDevices(otherAuth.User.UserID)
	if len(visible) != 1 || visible[0].DeviceID != "ios-1" {
		t.Fatalf("expected invited device visible to other user, got %+v", visible)
	}
	otherNetworkB, _, _, err := store.CreateNetwork(otherAuth.User.UserID, "测试组", "test", "test")
	if err != nil {
		t.Fatalf("create other network: %v", err)
	}
	if _, err := store.AddNetworkDevice(otherNetworkB.NetworkID, "ios-1", otherAuth.User.UserID, "", true); err != nil {
		t.Fatalf("expected visible device add to another network: %v", err)
	}
	if err := store.RemoveNetworkDevice(otherNetworkB.NetworkID, "ios-1"); err != nil {
		t.Fatalf("remove network device: %v", err)
	}
}

func TestDeviceBootstrapKeyCreatesDeviceSessionAndIsSingleUse(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	key, err := store.CreateDeviceBootstrapKey(auth.User.UserID, network.NetworkID, "edge box", 1800)
	if err != nil {
		t.Fatalf("create bootstrap key: %v", err)
	}
	if key.Key == "" || key.KeyHash == "" {
		t.Fatalf("expected clear key and stored hash, got %+v", key)
	}
	device, session, configs, err := store.BootstrapDeviceSession(key.Key, "linux-1", "Linux", "linux", "Linux", "6.0", "", "pub")
	if err != nil {
		t.Fatalf("bootstrap device session: %v", err)
	}
	if device.OwnerID != auth.User.UserID || device.Alias != "edge box" || device.GlobalIP == "" {
		t.Fatalf("unexpected bootstrapped device: %+v", device)
	}
	if session.DeviceToken == "" || session.State != "active" || session.UserID != auth.User.UserID {
		t.Fatalf("unexpected device session: %+v", session)
	}
	if session.DeviceTokenExpiresAt-session.RegisteredAt != int64(deviceSessionTTL.Seconds()) {
		t.Fatalf("expected device session ttl %s, got %+v", deviceSessionTTL, session)
	}
	if len(configs) == 0 || configs[0].NetworkID != network.NetworkID {
		t.Fatalf("expected network config for bootstrap network, got %+v", configs)
	}
	if _, _, _, err := store.BootstrapDeviceSession(key.Key, "linux-2", "Linux", "linux", "Linux", "6.0", "", "pub"); err != errNotFound {
		t.Fatalf("expected bootstrap key single use, got %v", err)
	}
	renewedDevice, renewedSession, renewedConfigs, err := store.RenewDeviceSession(session.DeviceToken, true, 10, 20)
	if err != nil {
		t.Fatalf("renew device session: %v", err)
	}
	if renewedDevice.DeviceID != device.DeviceID || renewedSession.SessionID != session.SessionID || len(renewedConfigs) == 0 {
		t.Fatalf("unexpected renewed session: device=%+v session=%+v configs=%+v", renewedDevice, renewedSession, renewedConfigs)
	}
	if renewedSession.DeviceTokenExpiresAt-renewedSession.LastRenewedAt != int64(deviceSessionTTL.Seconds()) {
		t.Fatalf("expected renewed device session ttl %s, got %+v", deviceSessionTTL, renewedSession)
	}
}

func TestRenewUserSessionExtendsExistingToken(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	store.mu.Lock()
	session := store.sessions[auth.Session.Token]
	session.ExpiresAt = time.Now().Unix() + 120
	store.sessions[auth.Session.Token] = session
	store.mu.Unlock()

	renewed, err := store.RenewUserSession(auth.Session.Token)
	if err != nil {
		t.Fatalf("renew user session: %v", err)
	}
	if renewed.Session.Token != auth.Session.Token {
		t.Fatalf("expected renew to keep token, got old=%s new=%s", auth.Session.Token, renewed.Session.Token)
	}
	if renewed.Session.ExpiresAt <= session.ExpiresAt {
		t.Fatalf("expected renewed session expiry to extend, before=%d after=%d", session.ExpiresAt, renewed.Session.ExpiresAt)
	}
	if renewed.Session.ExpiresAt-time.Now().Unix() > int64(userSessionTTL.Seconds()) {
		t.Fatalf("expected renewed session ttl to be at most %s, got %+v", userSessionTTL, renewed.Session)
	}
}

func TestLogoutWithUserSessionRevokesUserDeviceSessions(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	_, deviceSession, _, err := store.BindDeviceSession(auth.Session.Token, "mac-logout-1", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("bind device session: %v", err)
	}
	if err := store.LogoutSessions(auth.Session.Token, ""); err != nil {
		t.Fatalf("logout sessions: %v", err)
	}
	if _, _, _, err := store.RenewDeviceSession(deviceSession.DeviceToken, true, 1, 1); err != errUnauthorized {
		t.Fatalf("expected logged out device session to be revoked, got %v", err)
	}
	if _, err := store.AuthByToken(auth.Session.Token); err != errNotFound {
		t.Fatalf("expected user session removed, got %v", err)
	}
}

func TestUserSessionsAreUniqueWithinSameSecond(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	first := store.createSessionLocked(auth.User.UserID, 1234567890)
	second := store.createSessionLocked(auth.User.UserID, 1234567890)
	if first.SessionID == second.SessionID {
		t.Fatalf("expected unique session ids in same second, got %s", first.SessionID)
	}
	if first.Token == second.Token {
		t.Fatalf("expected unique tokens in same second, got %s", first.Token)
	}
}

func TestDeviceLoginForDeviceRequiresValidBrowserSessionToken(t *testing.T) {
	store := NewStore()
	alice, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	if _, _, err := store.RegisterDevice(alice.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a"); err != nil {
		t.Fatalf("register alice device: %v", err)
	}

	if _, err := store.CompleteDeviceLoginForDevice("mac-1", "stale-token", "login"); err != errNotFound {
		t.Fatalf("expected stale token to be rejected, got %v", err)
	}

	payload, err := store.CompleteDeviceLoginForDevice("mac-1", alice.Session.Token, "login")
	if err != nil {
		t.Fatalf("complete device login: %v", err)
	}
	if payload.UserID != alice.User.UserID || payload.UserLabel != alice.User.Email {
		t.Fatalf("expected device login to use token identity, got %+v", payload)
	}
	if payload.AccessToken != alice.Session.Token || payload.UserToken != alice.Session.Token {
		t.Fatalf("expected mqtt login payload to include fresh user token, got %+v", payload)
	}
	if payload.DeviceID == nil || *payload.DeviceID != "mac-1" {
		t.Fatalf("expected device id in payload, got %+v", payload)
	}
	device, err := store.GetDevice("mac-1")
	if err != nil {
		t.Fatalf("expected registered device to remain available: %v", err)
	}
	if device.OwnerID != alice.User.UserID {
		t.Fatalf("expected device owner %s, got %s", alice.User.UserID, device.OwnerID)
	}
	configs, err := store.NetworkConfigsForDevice("mac-1")
	if err != nil {
		t.Fatalf("expected registered device network config: %v", err)
	}
	if len(configs) == 0 {
		t.Fatalf("expected registered device to join default network")
	}
}

func TestDeviceLoginForDeviceRejectsMissingDevice(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}

	if _, err := store.CompleteDeviceLoginForDevice("missing-device", auth.Session.Token, "login"); err != errNotFound {
		t.Fatalf("expected missing device to be rejected, got %v", err)
	}
	if _, err := store.GetDevice("missing-device"); err != errNotFound {
		t.Fatalf("expected missing device to remain absent, got %v", err)
	}
}

func TestDeviceLoginPrepareRegistersPreloginDevice(t *testing.T) {
	store := NewStore()
	alice, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}

	prelogin, err := store.PrepareDeviceLoginDevice("mac-prelogin-1", "Mac", "macos", "macOS", "15.0", "", "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("prepare device login: %v", err)
	}
	if prelogin.DeviceID != "mac-prelogin-1" {
		t.Fatalf("expected prepared device id, got %+v", prelogin)
	}
	if prelogin.OwnerID != "" {
		t.Fatalf("expected prelogin device to be unbound, got %+v", prelogin)
	}
	if prelogin.GlobalIP != "" {
		t.Fatalf("expected prelogin device to not allocate ip before binding, got %+v", prelogin)
	}

	payload, err := store.CompleteDeviceLoginForDevice("mac-prelogin-1", alice.Session.Token, "login")
	if err != nil {
		t.Fatalf("complete device login: %v", err)
	}
	if payload.UserID != alice.User.UserID {
		t.Fatalf("expected alice login payload, got %+v", payload)
	}
	device, err := store.GetDevice("mac-prelogin-1")
	if err != nil {
		t.Fatalf("get bound device: %v", err)
	}
	if device.OwnerID != alice.User.UserID {
		t.Fatalf("expected prelogin device to bind to alice, got %+v", device)
	}
	if device.GlobalIP == "" {
		t.Fatalf("expected bound prelogin device to allocate ip, got %+v", device)
	}
	configs, err := store.NetworkConfigsForDevice("mac-prelogin-1")
	if err != nil {
		t.Fatalf("network configs: %v", err)
	}
	if len(configs) == 0 {
		t.Fatalf("expected bound prelogin device to join default network")
	}
}

func TestDeviceLoginAllowsSameUserMultipleDevices(t *testing.T) {
	store := NewStore()
	alice, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	prepared := []struct {
		deviceID  string
		name      string
		platform  string
		publicKey string
	}{
		{deviceID: "alice-mac-prelogin", name: "Alice Mac", platform: "macos", publicKey: "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{deviceID: "alice-android-prelogin", name: "Alice Android", platform: "android", publicKey: "pk_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}

	for _, item := range prepared {
		if _, err := store.PrepareDeviceLoginDevice(item.deviceID, item.name, item.platform, item.platform, "1.0", "", item.publicKey); err != nil {
			t.Fatalf("prepare %s: %v", item.deviceID, err)
		}
		payload, err := store.CompleteDeviceLoginForDevice(item.deviceID, alice.Session.Token, "login")
		if err != nil {
			t.Fatalf("complete %s: %v", item.deviceID, err)
		}
		if payload.UserID != alice.User.UserID {
			t.Fatalf("expected %s to bind to alice, got %+v", item.deviceID, payload)
		}
	}

	for _, item := range prepared {
		device, err := store.GetDevice(item.deviceID)
		if err != nil {
			t.Fatalf("get %s: %v", item.deviceID, err)
		}
		if device.OwnerID != alice.User.UserID {
			t.Fatalf("expected %s owner %s, got %+v", item.deviceID, alice.User.UserID, device)
		}
		configs, err := store.NetworkConfigsForDevice(item.deviceID)
		if err != nil {
			t.Fatalf("network configs for %s: %v", item.deviceID, err)
		}
		if len(configs) == 0 {
			t.Fatalf("expected %s to join alice default network", item.deviceID)
		}
	}
}

func TestDeviceLoginPrepareDoesNotOverwriteBoundDevice(t *testing.T) {
	store := NewStore()
	alice, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	if _, _, err := store.RegisterDevice(alice.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "Alice Mac", "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatalf("register alice device: %v", err)
	}

	if _, err := store.PrepareDeviceLoginDevice("mac-1", "Attacker", "linux", "Linux", "6.0", "bad", "pk_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); err != errConflict {
		t.Fatalf("expected public key mismatch to conflict, got %v", err)
	}
	if _, err := store.PrepareDeviceLoginDevice("mac-1", "Other Name", "linux", "Linux", "6.0", "bad", "pub-a"); err != errBadRequest {
		t.Fatalf("expected weak public key to be rejected, got %v", err)
	}
	if _, err := store.PrepareDeviceLoginDevice("mac-1", "Other Name", "linux", "Linux", "6.0", "bad", "pk_cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"); err != errConflict {
		t.Fatalf("expected strong non-matching device identity to conflict, got %v", err)
	}
	if _, err := store.PrepareDeviceLoginDevice("mac-1", "Other Name", "linux", "Linux", "6.0", "bad", "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatalf("expected matching device identity to prepare: %v", err)
	}
	device, err := store.GetDevice("mac-1")
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.Name != "Mac" || device.Platform != "macos" || device.Alias != "Alice Mac" {
		t.Fatalf("expected bound device metadata to remain unchanged, got %+v", device)
	}
}

func TestDeviceLoginPrepareExpiresOldPreloginDevices(t *testing.T) {
	store := NewStore()
	oldDevice, err := store.PrepareDeviceLoginDevice("old-prelogin", "Old", "linux", "Linux", "1.0", "", "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("prepare old device: %v", err)
	}
	freshDevice, err := store.PrepareDeviceLoginDevice("fresh-prelogin", "Fresh", "linux", "Linux", "1.0", "", "pk_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("prepare fresh device: %v", err)
	}
	store.mu.Lock()
	oldDevice.CreatedAt = time.Now().Add(-preloginDeviceTTL - time.Minute).Unix()
	oldDevice.UpdatedAt = oldDevice.CreatedAt
	store.devices[oldDevice.DeviceID] = oldDevice
	freshDevice.CreatedAt = time.Now().Unix()
	freshDevice.UpdatedAt = freshDevice.CreatedAt
	store.devices[freshDevice.DeviceID] = freshDevice
	store.mu.Unlock()

	if _, err := store.PrepareDeviceLoginDevice("new-prelogin", "New", "linux", "Linux", "1.0", "", "pk_cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"); err != nil {
		t.Fatalf("prepare new device: %v", err)
	}
	if _, err := store.GetDevice("old-prelogin"); err != errNotFound {
		t.Fatalf("expected old prelogin device expired, got %v", err)
	}
	if _, err := store.GetDevice("fresh-prelogin"); err != nil {
		t.Fatalf("expected fresh prelogin device retained: %v", err)
	}
}

func TestDeviceLoginForDeviceRejectsExistingDeviceOwnedByAnotherUser(t *testing.T) {
	store := NewStore()
	alice, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	bob, _, err := store.RegisterUser("bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	if _, _, err := store.RegisterDevice(alice.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a"); err != nil {
		t.Fatalf("register alice device: %v", err)
	}
	if _, err := store.CompleteDeviceLoginForDevice("mac-1", bob.Session.Token, "login"); err != errConflict {
		t.Fatalf("expected existing device owned by another user to conflict, got %v", err)
	}
	device, err := store.GetDevice("mac-1")
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.OwnerID != alice.User.UserID {
		t.Fatalf("expected owner to remain alice, got %+v", device)
	}
}

func TestChangeUserPassword(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	if !strings.HasPrefix(auth.User.PasswordHash, "$2") {
		t.Fatalf("expected new user password to use bcrypt, got %q", auth.User.PasswordHash)
	}
	if err := store.ChangeUserPassword(auth.User.UserID, "bad", "new-secret"); err != errBadRequest {
		t.Fatalf("expected bad old password error, got %v", err)
	}
	if err := store.ChangeUserPassword(auth.User.UserID, "secret", "new-secret"); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, err := store.LoginUser("alice@example.com", "secret"); err != errBadRequest {
		t.Fatalf("expected old password to fail, got %v", err)
	}
	if _, err := store.LoginUser("alice@example.com", "new-secret"); err != nil {
		t.Fatalf("expected new password login: %v", err)
	}
}

func TestLegacyPasswordHashCanLoginAndUpgradeOnChange(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("legacy@example.com", "secret", "Legacy")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	store.mu.Lock()
	user := store.users[auth.User.UserID]
	user.PasswordHash = legacyPasswordHash("secret")
	store.users[user.UserID] = user
	store.mu.Unlock()

	if _, err := store.LoginUser("legacy@example.com", "secret"); err != nil {
		t.Fatalf("expected legacy password login: %v", err)
	}
	if err := store.ChangeUserPassword(auth.User.UserID, "secret", "new-secret"); err != nil {
		t.Fatalf("change legacy password: %v", err)
	}
	store.mu.Lock()
	updatedHash := store.users[auth.User.UserID].PasswordHash
	store.mu.Unlock()
	if !strings.HasPrefix(updatedHash, "$2") {
		t.Fatalf("expected password change to upgrade hash, got %q", updatedHash)
	}
	if _, err := store.LoginUser("legacy@example.com", "new-secret"); err != nil {
		t.Fatalf("expected upgraded password login: %v", err)
	}
}

func TestUserLoginRateLimitBlocksRepeatedFailuresAndClearsOnSuccess(t *testing.T) {
	store := NewStore()
	if _, _, err := store.RegisterUser("rate@example.com", "secret", "Rate"); err != nil {
		t.Fatalf("register user: %v", err)
	}
	for i := 0; i < loginRateLimitMaxFail; i++ {
		if _, err := store.LoginUserWithRateLimit("rate@example.com", "bad", "203.0.113.10"); err != errBadRequest {
			t.Fatalf("expected bad password failure %d, got %v", i, err)
		}
	}
	if _, err := store.LoginUserWithRateLimit("rate@example.com", "secret", "203.0.113.10"); err != errRateLimited {
		t.Fatalf("expected login to be rate limited, got %v", err)
	}
	if _, err := store.LoginUserWithRateLimit("rate@example.com", "secret", "203.0.113.11"); err != nil {
		t.Fatalf("expected different IP login to succeed: %v", err)
	}
	if _, err := store.LoginUserWithRateLimit("rate@example.com", "bad", "203.0.113.11"); err != errBadRequest {
		t.Fatalf("expected failure after success to start a new window, got %v", err)
	}
	events := store.ListAuditEvents()
	if len(events) < loginRateLimitMaxFail+3 {
		t.Fatalf("expected login audit events, got %+v", events)
	}
	var sawRateLimited bool
	var sawSucceeded bool
	for _, event := range events {
		if event.Action != "auth.login" {
			continue
		}
		if event.Status == "rate_limited" {
			sawRateLimited = true
		}
		if event.Status == "succeeded" && event.ActorID != "" && event.RemoteIP == "203.0.113.11" {
			sawSucceeded = true
		}
	}
	if !sawRateLimited || !sawSucceeded {
		t.Fatalf("expected rate_limited and succeeded audit events, got %+v", events)
	}
}

func TestDeviceLoginPrepareRateLimitBlocksRepeatedAttempts(t *testing.T) {
	store := NewStore()
	for i := 0; i < prepareRateLimitMax; i++ {
		if err := store.CheckDeviceLoginPrepareRateLimit("device-1", "203.0.113.30"); err != nil {
			t.Fatalf("expected prepare attempt %d allowed, got %v", i, err)
		}
	}
	if err := store.CheckDeviceLoginPrepareRateLimit("device-1", "203.0.113.30"); err != errRateLimited {
		t.Fatalf("expected prepare attempts to be rate limited, got %v", err)
	}
	if err := store.CheckDeviceLoginPrepareRateLimit("device-1", "203.0.113.31"); err != nil {
		t.Fatalf("expected different IP allowed, got %v", err)
	}
	if err := store.CheckDeviceLoginPrepareRateLimit("device-2", "203.0.113.30"); err != nil {
		t.Fatalf("expected different device allowed, got %v", err)
	}
}

func TestOperatorLoginRateLimitBlocksRepeatedFailures(t *testing.T) {
	store := NewStore()
	for i := 0; i < loginRateLimitMaxFail; i++ {
		if _, err := store.LoginOperatorWithRateLimit("admin1", "bad", "203.0.113.20"); err != errBadRequest {
			t.Fatalf("expected bad ops password failure %d, got %v", i, err)
		}
	}
	if _, err := store.LoginOperatorWithRateLimit("admin1", "admin1", "203.0.113.20"); err != errRateLimited {
		t.Fatalf("expected ops login to be rate limited, got %v", err)
	}
	events := store.ListAuditEvents()
	if len(events) < loginRateLimitMaxFail+1 {
		t.Fatalf("expected ops login audit events, got %+v", events)
	}
	var sawOpsRateLimited bool
	for _, event := range events {
		if event.Action == "ops.auth.login" && event.Status == "rate_limited" {
			sawOpsRateLimited = true
			break
		}
	}
	if !sawOpsRateLimited {
		t.Fatalf("expected ops audit event to be rate limited, got %+v", events)
	}
}

func TestAuditEventDetailsRedactSensitiveFields(t *testing.T) {
	store := NewStore()
	event := store.RecordAuditEvent(AuditEvent{
		ActorType: "user",
		Action:    "test.audit",
		Status:    "succeeded",
		Details: map[string]string{
			"password":      "secret",
			"authorization": "Bearer secret",
			"accessToken":   "token",
			"mqttSecret":    "secret",
			"regularField":  "visible",
		},
	})
	if event.Details["password"] != "[redacted]" || event.Details["authorization"] != "[redacted]" || event.Details["accesstoken"] != "[redacted]" || event.Details["mqttsecret"] != "[redacted]" {
		t.Fatalf("expected sensitive audit details redacted, got %+v", event.Details)
	}
	if event.Details["regularfield"] != "visible" {
		t.Fatalf("expected regular audit detail preserved, got %+v", event.Details)
	}
}

func TestMQTTRequestKeysRedactsSensitiveFieldNames(t *testing.T) {
	keys := mqttRequestKeys(map[string]any{
		"clientId":      "device",
		"password":      "secret",
		"accessToken":   "token",
		"Authorization": "Bearer token",
	})
	joined := strings.Join(keys, ",")
	if strings.Contains(strings.ToLower(joined), "password") || strings.Contains(strings.ToLower(joined), "token") || strings.Contains(strings.ToLower(joined), "authorization") {
		t.Fatalf("expected sensitive mqtt request keys redacted, got %v", keys)
	}
	if !containsString(keys, "clientId") || !containsString(keys, "[redacted]") {
		t.Fatalf("expected public and redacted keys, got %v", keys)
	}
}

func TestRenewDeviceRefreshesRuntimeAndConfigs(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	renewed, configs, err := store.RenewDevice(device.DeviceID, auth.User.UserID, true, 123, 456)
	if err != nil {
		t.Fatalf("renew device: %v", err)
	}
	if renewed.DeviceID != device.DeviceID || len(configs) == 0 {
		t.Fatalf("unexpected renew response device=%+v configs=%+v", renewed, configs)
	}
	status := store.runtimeStatuses[device.DeviceID]
	if !status.HeartbeatOnline || !status.NetworkEnabled || status.RxBytesTotal != 123 || status.TxBytesTotal != 456 {
		t.Fatalf("runtime status not refreshed: %+v", status)
	}
}

func TestRelayCandidatesAndTicketRequireNetworkMembership(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	deviceA, _, _ := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	deviceB, _, _ := store.RegisterDevice(auth.User.UserID, "ios-1", "iPhone", "ios", "iOS", "18.0", "", "pub-b")
	candidates, err := store.RelayCandidates(network.NetworkID, deviceA.DeviceID)
	if err != nil {
		t.Fatalf("relay candidates: %v", err)
	}
	if len(candidates) == 0 || candidates[0].Transport != "udp" {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	ticket, err := store.IssueRelayTicket(network.NetworkID, "node-"+deviceA.DeviceID, "node-"+deviceB.DeviceID, "", []string{candidates[0].EndpointID})
	if err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	if ticket.NetworkID != network.NetworkID || ticket.SessionKey == "" || ticket.Signature == "" || ticket.RelayURL == "" {
		t.Fatalf("unexpected relay ticket: %+v", ticket)
	}
	if _, err := store.IssueRelayTicket(network.NetworkID, "node-"+deviceA.DeviceID, "node-missing", "", nil); err != errNotFound {
		t.Fatalf("expected missing peer not found, got %v", err)
	}
}

func TestRelayCandidatesDoNotFallbackAfterNodesDeleted(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("relay-empty@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := store.RegisterDevice(auth.User.UserID, "mac-empty-relay", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	store.mu.Lock()
	store.relayNodes = map[string]OpsRelayNode{}
	store.mu.Unlock()

	candidates, err := store.RelayCandidates(network.NetworkID, device.DeviceID)
	if err != nil {
		t.Fatalf("relay candidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected deleted relay nodes to stay empty, got %+v", candidates)
	}
}

func TestActiveRelayCandidatesFollowPriorityAndHealthyOnly(t *testing.T) {
	store := NewStore()
	store.mu.Lock()
	store.relayNodes = map[string]OpsRelayNode{}
	store.addRelayNodeLocked(OpsRelayNode{Name: "slow", Region: "cn", Transport: "relay_udp", PublicAddr: "udp://slow.example.com:29110", Status: "active", Health: "healthy", Priority: 100})
	store.addRelayNodeLocked(OpsRelayNode{Name: "fast", Region: "cn", Transport: "relay_udp", PublicAddr: "udp://fast.example.com:29110", Status: "active", Health: "healthy", Priority: 10})
	store.addRelayNodeLocked(OpsRelayNode{Name: "warning", Region: "cn", Transport: "relay_udp", PublicAddr: "udp://warning.example.com:29110", Status: "active", Health: "warning", Priority: 1})
	candidates := store.activeRelayCandidatesLocked()
	store.mu.Unlock()

	if len(candidates) != 2 {
		t.Fatalf("expected only healthy candidates, got %+v", candidates)
	}
	if candidates[0].Address != "fast.example.com:29110" || candidates[1].Address != "slow.example.com:29110" {
		t.Fatalf("expected priority order, got %+v", candidates)
	}
}

func TestRelayURLUsesDERPSchemeForDERPCandidates(t *testing.T) {
	candidate := RelayCandidate{
		Transport: "derp_tcp_tls_443",
		Address:   "derp.example.com:29120",
	}
	if got := relayURL(candidate); got != "derp://derp.example.com:29120" {
		t.Fatalf("unexpected derp relay url: %s", got)
	}
}

func TestRelayTicketCandidateIsStablePerSession(t *testing.T) {
	candidates := []RelayCandidate{
		{EndpointID: "derp-a", Transport: "derp_tcp_tls_443", Address: "derp-a.example.com:29120"},
		{EndpointID: "derp-b", Transport: "derp_tcp_tls_443", Address: "derp-b.example.com:29122"},
	}
	sessionID := relaySessionID("net-1", "node-a", "node-b")
	fromA := chooseRelayCandidate(candidates, []string{"derp-a"}, sessionID)
	fromB := chooseRelayCandidate(candidates, []string{"derp-b"}, sessionID)

	if fromA.EndpointID == "" || fromA.EndpointID != fromB.EndpointID {
		t.Fatalf("expected stable candidate for both peers, fromA=%+v fromB=%+v", fromA, fromB)
	}
}

func TestRelayTicketCandidateHonorsPreferredTransport(t *testing.T) {
	candidates := []RelayCandidate{
		{EndpointID: "derp-a", Transport: "derp_tcp_tls_443", Address: "derp-a.example.com:29120"},
		{EndpointID: "relay-a", Transport: "udp", Address: "relay-a.example.com:29112"},
		{EndpointID: "relay-b", Transport: "udp", Address: "relay-b.example.com:29114"},
	}
	sessionID := relaySessionID("net-1", "node-a", "node-b")
	fromA := chooseRelayCandidate(candidates, []string{"relay-a"}, sessionID)
	fromB := chooseRelayCandidate(candidates, []string{"relay-b"}, sessionID)

	if fromA.Transport != "udp" || fromB.Transport != "udp" || fromA.EndpointID != fromB.EndpointID {
		t.Fatalf("expected stable UDP candidate for both peers, fromA=%+v fromB=%+v", fromA, fromB)
	}
}

func TestPunchConnectRequiresNetworkMembership(t *testing.T) {
	store := NewStore()
	auth, network, err := store.RegisterUser("punch-membership@example.com", "secret", "Punch")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	deviceA, _, _ := store.RegisterDevice(auth.User.UserID, "punch-mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	deviceB, _, _ := store.RegisterDevice(auth.User.UserID, "punch-ios-1", "iPhone", "ios", "iOS", "18.0", "", "pub-b")
	mqttCfg := MQTTConfig{Enabled: true, UsernamePrefix: "slan", PasswordSecret: "test-secret", CredentialTTLSeconds: 3600}
	mqttCredential := deviceMQTTCredential(mqttCfg, deviceA.DeviceID, time.Now())
	punchAuth := punchDeviceAuth{
		DeviceID:  deviceA.DeviceID,
		Username:  mqttCredential.Username,
		Signature: punchMQTTSignature(deviceA.DeviceID, mqttCredential.Password),
	}
	if err := store.AuthorizePunchConnect(network.NetworkID, "node-"+deviceA.DeviceID, "node-"+deviceB.DeviceID, punchAuth, mqttCfg); err != nil {
		t.Fatalf("authorize punch connect: %v", err)
	}
	if err := store.AuthorizePunchConnect(network.NetworkID, "", "node-"+deviceB.DeviceID, punchAuth, mqttCfg); err != errBadRequest {
		t.Fatalf("expected bad request for missing requester, got %v", err)
	}
	if err := store.AuthorizePunchConnect(network.NetworkID, "node-"+deviceA.DeviceID, "node-missing", punchAuth, mqttCfg); err != errNotFound {
		t.Fatalf("expected missing peer not found, got %v", err)
	}
	if err := store.AuthorizePunchConnect(network.NetworkID, "node-"+deviceB.DeviceID, "node-"+deviceA.DeviceID, punchAuth, mqttCfg); err != errUnauthorized {
		t.Fatalf("expected requester token mismatch unauthorized, got %v", err)
	}
}

func TestOpsLoginAssignPlanAndQuota(t *testing.T) {
	store := NewStore()
	operatorAuth, err := store.LoginOperator("admin1", "admin1")
	if err != nil {
		t.Fatalf("operator login: %v", err)
	}
	if operatorAuth.Session.Token == "" || operatorAuth.Operator.OperatorID == "" {
		t.Fatalf("unexpected operator auth: %+v", operatorAuth)
	}
	auth, _, err := store.RegisterUser("quota@example.com", "secret", "Quota")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	expiresAt := time.Now().Add(30 * 24 * time.Hour).Unix()
	customer, renewal, err := store.AssignCustomerPlan(auth.User.UserID, "pro", expiresAt, 39, "monthly", operatorAuth.Operator.Email)
	if err != nil {
		t.Fatalf("assign plan: %v", err)
	}
	if customer.PlanCode != "pro" || renewal.PlanCode != "pro" {
		t.Fatalf("unexpected plan assignment: %+v %+v", customer, renewal)
	}
	quota, err := store.DeviceQuota(auth.User.UserID)
	if err != nil {
		t.Fatalf("quota: %v", err)
	}
	if quota.PlanCode != "pro" || quota.TotalDeviceLimit != 130 || quota.RemainingDevices != 130 {
		t.Fatalf("unexpected quota: %+v", quota)
	}
}

func TestDeviceInviteRespectsTotalDeviceLimit(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("free@example.com", "secret", "Free")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, _, err := store.RegisterDevice(auth.User.UserID, "dev-"+stringID(i), "Device", "test", "test", "1", "", ""); err != nil {
			t.Fatalf("register device %d: %v", i, err)
		}
	}
	if quota, err := store.DeviceQuota(auth.User.UserID); err != nil || quota.RemainingDevices != 0 {
		t.Fatalf("expected exhausted quota, got %+v err=%v", quota, err)
	}
	if _, err := store.CreateDeviceInvite(auth.User.UserID, 0); err != errConflict {
		t.Fatalf("expected invite conflict when quota exhausted, got %v", err)
	}
}

func TestConsoleLoginKeyIsSingleUseAndUserScoped(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("console@example.com", "secret", "Console")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	if _, _, err := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "14", "", "pk_console_1"); err != nil {
		t.Fatalf("register device: %v", err)
	}
	key, err := store.CreateConsoleLoginKey(auth.Session.Token, "mac-1", 2*time.Minute)
	if err != nil {
		t.Fatalf("create console login key: %v", err)
	}
	if key.LoginKey == "" || key.UserID != auth.User.UserID || key.DeviceID != "mac-1" || key.Status != "unused" {
		t.Fatalf("unexpected console login key: %+v", key)
	}
	consumed, err := store.ConsumeConsoleLoginKey(key.LoginKey)
	if err != nil {
		t.Fatalf("consume console login key: %v", err)
	}
	if consumed.User.UserID != auth.User.UserID || consumed.Session.Token == "" || consumed.Session.UserID != auth.User.UserID {
		t.Fatalf("unexpected console auth response: %+v", consumed)
	}
	if _, err := store.ConsumeConsoleLoginKey(key.LoginKey); err != errNotFound {
		t.Fatalf("expected used console login key to be rejected, got %v", err)
	}
}

func TestConsoleLoginKeyRejectsInvalidOrExpiredCredentials(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("expired-console@example.com", "secret", "Expired")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	otherAuth, _, err := store.RegisterUser("other-console@example.com", "secret", "Other")
	if err != nil {
		t.Fatalf("register other user: %v", err)
	}
	if _, _, err := store.RegisterDevice(otherAuth.User.UserID, "other-mac", "Other Mac", "macos", "macOS", "14", "", "pk_console_other"); err != nil {
		t.Fatalf("register other device: %v", err)
	}
	if _, err := store.CreateConsoleLoginKey("bad-token", "mac-1", 2*time.Minute); err != errNotFound {
		t.Fatalf("expected invalid session token to be not found, got %v", err)
	}
	if _, err := store.CreateConsoleLoginKey(auth.Session.Token, "", 2*time.Minute); err != errBadRequest {
		t.Fatalf("expected empty console device id to be bad request, got %v", err)
	}
	if _, err := store.CreateConsoleLoginKey(auth.Session.Token, "missing-device", 2*time.Minute); err != errNotFound {
		t.Fatalf("expected missing console device to be not found, got %v", err)
	}
	if _, err := store.CreateConsoleLoginKey(auth.Session.Token, "other-mac", 2*time.Minute); err != errNotFound {
		t.Fatalf("expected foreign console device to be not found, got %v", err)
	}
	if _, _, err := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "14", "", "pk_console_1"); err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.ConsumeConsoleLoginKey("not-a-key"); err != errNotFound {
		t.Fatalf("expected unknown console login key to be not found, got %v", err)
	}
	key, err := store.CreateConsoleLoginKey(auth.Session.Token, "mac-1", 2*time.Minute)
	if err != nil {
		t.Fatalf("create console login key: %v", err)
	}
	store.mu.Lock()
	key.ExpiresAt = time.Now().Unix() - 1
	store.consoleLoginKeys[key.LoginKey] = key
	store.mu.Unlock()
	if _, err := store.ConsumeConsoleLoginKey(key.LoginKey); err != errNotFound {
		t.Fatalf("expected expired console login key to be rejected, got %v", err)
	}
}

func stringID(v int) string {
	if v == 0 {
		return "0"
	}
	const alphabet = "0123456789"
	buf := make([]byte, 0, 8)
	for v > 0 {
		buf = append([]byte{alphabet[v%10]}, buf...)
		v /= 10
	}
	return string(buf)
}
