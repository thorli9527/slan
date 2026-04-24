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
