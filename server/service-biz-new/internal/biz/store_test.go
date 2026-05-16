package biz

import (
	"testing"
	"time"
)

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
	if device.GlobalIP != "10.0.0.1" || device.GlobalName != "mac-1.staticlss.com" {
		t.Fatalf("unexpected global device identity: %+v", device)
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
	bobDevice, bobSession, _, err := store.BindDeviceSession(bobAuth.Session.Token, "same-client", "Mac", "macos", "macOS", "15.0", "", "pub-b")
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
	if len(config.Rules) != 1 || config.Rules[0].RuleID != rule.RuleID {
		t.Fatalf("unexpected acl: %+v", config.Rules)
	}
	if len(config.DNSRecords) != 1 || config.DNSRecords[0].RecordID != record.RecordID {
		t.Fatalf("unexpected dns: %+v", config.DNSRecords)
	}
}

func TestReportDeviceRuntimeMarksPeerActiveForNetworkConfig(t *testing.T) {
	store := NewStore()
	auth, network, _ := store.RegisterUser("alice@example.com", "secret", "Alice")
	deviceA, _, _ := store.RegisterDevice(auth.User.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	deviceB, _, _ := store.RegisterDevice(auth.User.UserID, "ios-1", "iPhone", "ios", "iOS", "18.0", "", "pub-b")

	config, err := store.NetworkConfig(network.NetworkID, deviceA.DeviceID)
	if err != nil {
		t.Fatalf("network config before runtime report: %v", err)
	}
	if len(config.Peers) != 0 {
		t.Fatalf("expected inactive peer to be filtered, got %+v", config.Peers)
	}

	store.ReportDeviceRuntime(deviceB.DeviceID, true, 10, 20)
	config, err = store.NetworkConfig(network.NetworkID, deviceA.DeviceID)
	if err != nil {
		t.Fatalf("network config after runtime report: %v", err)
	}
	if len(config.Peers) != 1 || config.Peers[0].DeviceID != deviceB.DeviceID {
		t.Fatalf("expected active peer from runtime report, got %+v", config.Peers)
	}

	store.ReportDeviceRuntime(deviceB.DeviceID, false, 10, 20)
	config, err = store.NetworkConfig(network.NetworkID, deviceA.DeviceID)
	if err != nil {
		t.Fatalf("network config after runtime disabled: %v", err)
	}
	if len(config.Peers) != 0 {
		t.Fatalf("expected disabled runtime peer to be filtered, got %+v", config.Peers)
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
	cfg := MQTTConfig{Enabled: true, TopicPrefix: "slan/v1"}
	deviceID := "mac-1"
	if !mqttAllowTopicAccess(cfg, "device", deviceID, "slan/v1/devices/mac-1/heartbeat", false) {
		t.Fatalf("device heartbeat topic should be publishable")
	}
	if !mqttAllowTopicAccess(cfg, "device", deviceID, "slan/v1/devices/mac-1/runtime", false) {
		t.Fatalf("device runtime topic should be publishable")
	}
	if !mqttAllowTopicAccess(cfg, "device", deviceID, "slan/v1/devices/mac-1/control/down", true) {
		t.Fatalf("device control down topic should be subscribable")
	}
	if !mqttAllowTopicAccess(cfg, "server", "", "slan/v1/devices/mac-1/control/down", false) {
		t.Fatalf("server should publish control down topics")
	}
	if !mqttAllowTopicAccess(cfg, "server", "", "slan/v1/networks/net-1/broadcast", false) {
		t.Fatalf("server should publish network broadcast topics")
	}
	if mqttAllowTopicAccess(cfg, "device", deviceID, "slan/v1/devices/ios-1/heartbeat", false) {
		t.Fatalf("device must not publish another device heartbeat")
	}
}

func TestGlobalIPPoolPreGeneratesAndRefills(t *testing.T) {
	store := NewStore()
	subnets := store.ListIPAMSubnets()
	if len(subnets) != 1 || subnets[0].CIDRBlock != "10.0.0.0/20" || subnets[0].GeneratedCapacity != 4094 {
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
		if i == 0 && device.GlobalIP != "10.0.0.1" {
			t.Fatalf("expected first assigned ip 10.0.0.1, got %s", device.GlobalIP)
		}
	}
	subnets = store.ListIPAMSubnets()
	if len(subnets) != 2 || subnets[1].CIDRBlock != "10.0.16.0/20" {
		t.Fatalf("expected second subnet after low watermark refill, got %+v", subnets)
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

func TestDeviceLoginForDeviceRestoresBrowserSessionByEmailWithoutCallback(t *testing.T) {
	store := NewStore()
	alice, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	bob, _, err := store.RegisterUser("bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}

	payload, err := store.CompleteDeviceLoginForDevice("mac-1", "stale-token", "login", bob.User.UserID, alice.User.Email)
	if err != nil {
		t.Fatalf("complete device login: %v", err)
	}
	if payload.UserID != alice.User.UserID || payload.UserLabel != alice.User.Email {
		t.Fatalf("expected device login restored by email without callback, got %+v", payload)
	}
	if payload.DeviceID == nil || *payload.DeviceID != "mac-1" {
		t.Fatalf("expected device id in payload, got %+v", payload)
	}
	device, err := store.GetDevice("mac-1")
	if err != nil {
		t.Fatalf("expected browser login to create missing device: %v", err)
	}
	if device.OwnerID != alice.User.UserID {
		t.Fatalf("expected created device owner %s, got %s", alice.User.UserID, device.OwnerID)
	}
	configs, err := store.NetworkConfigsForDevice("mac-1")
	if err != nil {
		t.Fatalf("expected created device network config: %v", err)
	}
	if len(configs) == 0 {
		t.Fatalf("expected created device to join default network")
	}
}

func TestChangeUserPassword(t *testing.T) {
	store := NewStore()
	auth, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
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

func TestOpsLoginAssignPlanAndQuota(t *testing.T) {
	store := NewStore()
	operatorAuth, err := store.LoginOperator("admin@slan.local", "admin123456")
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
