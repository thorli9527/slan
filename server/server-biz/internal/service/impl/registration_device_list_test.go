package impl

import (
	"context"
	"testing"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func TestListDevicesIncludesVisibleNetworkJoinRequests(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now().Unix()

	for _, user := range []repo.User{
		{UserID: "owner-user", Email: "owner@example.com", PasswordHash: "hash", ActiveNetworkID: "net-1"},
		{UserID: "guest-user", Email: "guest@example.com", PasswordHash: "hash"},
	} {
		if err := state.pg.CreateUser(ctx, user); err != nil {
			t.Fatalf("create user %s: %v", user.UserID, err)
		}
	}
	if err := state.pg.CreateNetworkWithDefaultSubnet(ctx, "owner-user", dto.Network{
		NetworkID:         "net-1",
		Name:              "owner-net",
		DefaultSubnetID:   "subnet-1",
		DefaultSubnetCIDR: "10.0.0.0/16",
	}, dto.Subnet{
		SubnetID:          "subnet-1",
		NetworkID:         "net-1",
		Name:              "default",
		CIDR:              "10.0.0.0/16",
		GatewayIP:         "10.0.0.1",
		AllocationStartIP: "10.0.0.2",
		AllocationEndIP:   "10.0.255.254",
		IsDefault:         true,
		Status:            "active",
	}); err != nil {
		t.Fatalf("create network: %v", err)
	}
	for _, device := range []repo.Device{
		{DeviceID: "owner-device", UserID: "owner-user", MachineID: "owner-machine", Name: "owner pc", Platform: "windows", Status: "online", CreatedAt: now},
		{DeviceID: "guest-device", UserID: "guest-user", MachineID: "guest-machine", Name: "guest pc", Platform: "windows", Status: "offline", CreatedAt: now},
	} {
		if err := state.pg.InsertDevice(ctx, device); err != nil {
			t.Fatalf("insert device %s: %v", device.DeviceID, err)
		}
	}
	for _, member := range []dto.NetworkMember{
		{MemberID: "member-owner", NetworkID: "net-1", DeviceID: "owner-device", Role: "owner", CreatedAt: now, Status: "active"},
		{MemberID: "member-guest", NetworkID: "net-1", DeviceID: "guest-device", Role: "member", CreatedAt: now, Status: "pending"},
	} {
		if err := state.pg.CreateMember(ctx, member); err != nil {
			t.Fatalf("create member %s: %v", member.MemberID, err)
		}
	}

	devices, err := dbDeviceService{state: state}.ListByUser("owner-user")
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}

	byID := make(map[string]dto.Device, len(devices))
	for _, device := range devices {
		byID[device.DeviceID] = device
	}
	if len(byID) != 2 {
		t.Fatalf("expected owner and pending guest devices, got %+v", devices)
	}
	guest := byID["guest-device"]
	if guest.OwnerEmail != "guest@example.com" {
		t.Fatalf("expected guest owner email, got %+v", guest)
	}
	if guest.MembershipStatus != "pending" || guest.NetworkRole != "member" {
		t.Fatalf("expected pending member status, got %+v", guest)
	}
}

func TestListDevicesForMemberDoesNotExposeOtherNetworkDevices(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now().Unix()

	for _, user := range []repo.User{
		{UserID: "owner-user", Email: "owner@example.com", PasswordHash: "hash", ActiveNetworkID: "net-1"},
		{UserID: "member-user", Email: "member@example.com", PasswordHash: "hash", ActiveNetworkID: "net-1"},
	} {
		if err := state.pg.CreateUser(ctx, user); err != nil {
			t.Fatalf("create user %s: %v", user.UserID, err)
		}
	}
	createNetworkFixture(t, state, "owner-user", "net-1", "subnet-1", "10.0.0.0/16")
	for _, device := range []repo.Device{
		{DeviceID: "owner-device", UserID: "owner-user", MachineID: "owner-machine", Name: "owner pc", Platform: "windows", Status: "online", CreatedAt: now},
		{DeviceID: "member-device", UserID: "member-user", MachineID: "member-machine", Name: "member pc", Platform: "windows", Status: "offline", CreatedAt: now},
	} {
		if err := state.pg.InsertDevice(ctx, device); err != nil {
			t.Fatalf("insert device %s: %v", device.DeviceID, err)
		}
	}
	for _, member := range []dto.NetworkMember{
		{MemberID: "member-owner", NetworkID: "net-1", DeviceID: "owner-device", Role: "owner", CreatedAt: now, Status: "active"},
		{MemberID: "member-peer", NetworkID: "net-1", DeviceID: "member-device", Role: "member", CreatedAt: now, Status: "active"},
	} {
		if err := state.pg.CreateMember(ctx, member); err != nil {
			t.Fatalf("create member %s: %v", member.MemberID, err)
		}
	}

	devices, err := dbDeviceService{state: state}.ListByUser("member-user")
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}

	if len(devices) != 1 || devices[0].DeviceID != "member-device" {
		t.Fatalf("expected only member-owned devices, got %+v", devices)
	}
	if devices[0].MembershipStatus != "active" || devices[0].NetworkRole != "member" {
		t.Fatalf("expected own membership metadata, got %+v", devices[0])
	}
}

func TestRegisterNodeOnlyReturnsActiveNetworkIDs(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now().Unix()

	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:          "user-1",
		Email:           "user@example.com",
		PasswordHash:    "hash",
		ActiveNetworkID: "net-active",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "device",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("insert device: %v", err)
	}
	for _, networkID := range []string{"net-active", "net-pending", "net-rejected"} {
		createNetworkFixture(t, state, "user-1", networkID, "subnet-"+networkID, "10.0.0.0/16")
	}
	for _, member := range []dto.NetworkMember{
		{MemberID: "member-active", NetworkID: "net-active", DeviceID: "dev-1", Role: "owner", CreatedAt: now, Status: "active"},
		{MemberID: "member-pending", NetworkID: "net-pending", DeviceID: "dev-1", Role: "member", CreatedAt: now, Status: "pending"},
		{MemberID: "member-rejected", NetworkID: "net-rejected", DeviceID: "dev-1", Role: "member", CreatedAt: now, Status: "rejected"},
	} {
		if err := state.pg.CreateMember(ctx, member); err != nil {
			t.Fatalf("create member %s: %v", member.MemberID, err)
		}
	}

	node, err := dbNodeService{state: state}.Register("user-1", dto.RegisterNodeRequest{
		DeviceID:      "dev-1",
		NodeID:        "node-1",
		NodePublicKey: "node-public-key",
		Capabilities:  []string{"desktop"},
	})
	if err != nil {
		t.Fatalf("register node: %v", err)
	}
	if len(node.NetworkIDs) != 1 || node.NetworkIDs[0] != "net-active" {
		t.Fatalf("expected only active network ids, got %+v", node.NetworkIDs)
	}
}

func TestMarkMQTTReachableUpdatesControlReachabilityOnly(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now().Unix()

	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:          "user-1",
		Email:           "user@example.com",
		PasswordHash:    "hash",
		ActiveNetworkID: "net-1",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "10.0.0.0/16")
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "device",
		Platform:  "windows",
		Status:    "offline",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("insert device: %v", err)
	}
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-1",
		NetworkID: "net-1",
		DeviceID:  "dev-1",
		Role:      "owner",
		CreatedAt: now,
		Status:    "active",
	}); err != nil {
		t.Fatalf("create member: %v", err)
	}

	if err := (dbDeviceService{state: state}).MarkMQTTReachable("dev-1"); err != nil {
		t.Fatalf("mark mqtt reachable: %v", err)
	}

	device, err := state.pg.GetDeviceByID(ctx, "dev-1")
	if err != nil {
		t.Fatalf("load device: %v", err)
	}
	if device.Status != "offline" {
		t.Fatalf("expected legacy device status unchanged, got %s", device.Status)
	}
	got, err := state.pg.GetDeviceNetworkState(ctx, "dev-1", "net-1")
	if err != nil {
		t.Fatalf("load device network state: %v", err)
	}
	if !got.ControlReachable {
		t.Fatalf("expected control channel reachable, got %+v", got)
	}
	if got.NetworkOnline || got.TunnelUp || got.LastProbeOK {
		t.Fatalf("expected network state to stay offline, got %+v", got)
	}
}

func TestListDevicesRefreshesStalePresenceBeforeReturning(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now()

	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user@example.com",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-stale",
		UserID:    "user-1",
		MachineID: "machine-stale",
		Name:      "stale device",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: now.Unix(),
	}); err != nil {
		t.Fatalf("insert device: %v", err)
	}
	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-stale",
		UserID:           "user-1",
		DeviceID:         "dev-stale",
		NodeID:           "node-stale",
		NetworkID:        "net-stale",
		SessionToken:     "token-stale",
		ConnectedAt:      now.Add(-5 * time.Minute).Unix(),
		LastSeenAt:       now.Add(-5 * time.Minute).Unix(),
	}); err != nil {
		t.Fatalf("create stale control session: %v", err)
	}

	devices, err := dbDeviceService{state: state}.ListByUser("user-1")
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected one device, got %+v", devices)
	}
	if devices[0].Status != "offline" {
		t.Fatalf("expected stale device to be returned offline, got %+v", devices[0])
	}
}
