package biz

import "testing"

func TestRegisterUserAndDeviceJoinDefaultWorkspace(t *testing.T) {
	store := NewStore()
	auth, member, workspace, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	user := auth.User
	if workspace.WorkspaceID != "default-"+user.UserID || member.WorkspaceID != workspace.WorkspaceID || member.UserID != user.UserID {
		t.Fatalf("expected user to join default workspace, got %+v", member)
	}
	if workspace.Code != "default" {
		t.Fatalf("expected default workspace code, got %+v", workspace)
	}

	device, deviceMember, err := store.RegisterDevice(user.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "work mac", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if device.GlobalIP != "10.0.0.1" || device.GlobalName != "mac-1.vlan.com" {
		t.Fatalf("unexpected global device identity: %+v", device)
	}
	if deviceMember.WorkspaceID != workspace.WorkspaceID || !deviceMember.Enabled {
		t.Fatalf("expected device to join default workspace enabled, got %+v", deviceMember)
	}
}

func TestWorkspaceNetworkConfigReturnsPeersACLAndDNS(t *testing.T) {
	store := NewStore()
	auth, _, workspace, _ := store.RegisterUser("alice@example.com", "secret", "Alice")
	user := auth.User
	deviceA, _, _ := store.RegisterDevice(user.UserID, "mac-1", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	deviceB, _, _ := store.RegisterDevice(user.UserID, "ios-1", "iPhone", "ios", "iOS", "18.0", "", "pub-b")
	groups := store.ListSecurityGroups(workspace.WorkspaceID)
	if len(groups) == 0 {
		t.Fatal("expected default security group")
	}
	rule, err := store.AddSecurityGroupRule(groups[0].SecurityGroupID, "ingress", "allow", "tcp", "device", deviceA.DeviceID, "ssh", 100, 22, 22, true)
	if err != nil {
		t.Fatalf("add acl: %v", err)
	}
	zones := store.ListDNSZones(workspace.WorkspaceID)
	if len(zones) == 0 {
		t.Fatal("expected default dns zone")
	}
	record, err := store.AddDNSRecord(workspace.WorkspaceID, zones[0].ZoneID, "phone", "A", deviceB.DeviceID, "", "", "443", 60)
	if err != nil {
		t.Fatalf("add dns: %v", err)
	}

	config, err := store.NetworkConfig(workspace.WorkspaceID, deviceA.DeviceID)
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

func TestGlobalIPPoolPreGeneratesAndRefills(t *testing.T) {
	store := NewStore()
	subnets := store.ListIPAMSubnets()
	if len(subnets) != 1 || subnets[0].CIDRBlock != "10.0.0.0/20" || subnets[0].GeneratedCapacity != 4094 {
		t.Fatalf("unexpected initial ip pool: %+v", subnets)
	}

	auth, _, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
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

func TestDeviceInviteIsSingleUseAndExpiresInThirtyMinutes(t *testing.T) {
	store := NewStore()
	auth, _, workspace, err := store.RegisterUser("alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	_, _, err = store.RegisterDevice(auth.User.UserID, "ios-1", "iPhone", "ios", "iOS", "18.0", "Alice Phone", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := store.AddWorkspaceDevice(workspace.WorkspaceID, "ios-1", auth.User.UserID, "", true); err != errConflict {
		t.Fatalf("expected duplicate default workspace add conflict, got %v", err)
	}
	otherAuth, _, otherWorkspace, err := store.RegisterUser("bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register other user: %v", err)
	}
	if _, err := store.AddWorkspaceDevice(otherWorkspace.WorkspaceID, "ios-1", otherAuth.User.UserID, "", true); err != errBadRequest {
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
	otherWorkspaceB, _, _, _, err := store.CreateWorkspace(otherAuth.User.UserID, "测试组", "test", "test")
	if err != nil {
		t.Fatalf("create other workspace: %v", err)
	}
	if _, err := store.AddWorkspaceDevice(otherWorkspaceB.WorkspaceID, "ios-1", otherAuth.User.UserID, "", true); err != nil {
		t.Fatalf("expected visible device add to another workspace: %v", err)
	}
	if err := store.RemoveWorkspaceDevice(otherWorkspaceB.WorkspaceID, "ios-1"); err != nil {
		t.Fatalf("remove workspace device: %v", err)
	}
}

func TestChangeUserPassword(t *testing.T) {
	store := NewStore()
	auth, _, _, err := store.RegisterUser("alice@example.com", "secret", "Alice")
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
