package impl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestCreateNetwork_RejectsSecondOwnedNetwork(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.pg.CreateNetworkWithDefaultSubnet(ctx, "user-1", dto.Network{
		NetworkID:         "net-1",
		Name:              "primary",
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
		t.Fatalf("create existing network: %v", err)
	}

	_, err := dbNetworkService{state: state}.Create("user-1", dto.CreateNetworkRequest{
		Name: "secondary",
		CIDR: "10.1.0.0/16",
	})
	if !errors.Is(err, service.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}

	user, err := state.pg.GetUserByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if user.ActiveNetworkID != "" {
		t.Fatalf("expected active network to stay empty after rejected create, got %s", user.ActiveNetworkID)
	}
}

func TestRegister_DoesNotCreateDefaultOwnedNetwork(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()

	auth, err := dbAuthService{state: state}.Register(dto.RegisterRequest{
		Email:    "new-user@local.slan",
		Password: "Register-2026!",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	if auth.UserID == "" || auth.AccessToken == "" {
		t.Fatalf("expected auth tokens, got %+v", auth)
	}

	user, err := state.pg.GetUserByID(ctx, auth.UserID)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.ActiveNetworkID != "" {
		t.Fatalf("expected active network to stay empty, got %s", user.ActiveNetworkID)
	}

	if _, err := state.pg.GetOwnedNetworkByUser(ctx, auth.UserID); !repo.IsNotFound(err) {
		t.Fatalf("expected no owned network after register, got %v", err)
	}
}

func TestCreateNetwork_CreatesTemplatedSubnets(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	created, err := dbNetworkService{state: state}.Create("user-1", dto.CreateNetworkRequest{
		Name: "office",
		CIDR: "10.0.0.0/22",
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	if created.DefaultSubnetCIDR != "10.0.0.0/24" {
		t.Fatalf("expected default subnet /24, got %+v", created)
	}

	subnets, err := state.pg.ListSubnetsByNetwork(ctx, created.NetworkID)
	if err != nil {
		t.Fatalf("list subnets: %v", err)
	}
	if len(subnets) != 4 {
		t.Fatalf("expected four subnets, got %+v", subnets)
	}
	wantNames := []string{"总网络", "开发部", "营销部", "人事部"}
	wantCIDRs := []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"}
	for index, subnet := range subnets {
		if subnet.Name != wantNames[index] || subnet.CIDR != wantCIDRs[index] || subnet.Remark == "" {
			t.Fatalf("unexpected subnet at %d: %+v", index, subnet)
		}
	}
}

func TestCreateNetwork_AppliesDHCPOptionsToDefaultSubnet(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	created, err := dbNetworkService{state: state}.Create("user-1", dto.CreateNetworkRequest{
		Name:              "office",
		CIDR:              "10.8.0.0/24",
		AllocationStartIP: "10.8.0.2",
		AllocationEndIP:   "10.8.0.62",
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}

	subnet, err := state.pg.GetSubnetByID(ctx, created.DefaultSubnetID)
	if err != nil {
		t.Fatalf("load default subnet: %v", err)
	}
	if subnet.AllocationStartIP != "10.8.0.2" ||
		subnet.AllocationEndIP != "10.8.0.62" {
		t.Fatalf("expected custom dhcp options, got %+v", subnet)
	}
}

func TestRegisterDevice_DoesNotProvisionIntoNetwork(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()

	auth, err := dbAuthService{state: state}.Register(dto.RegisterRequest{
		Email:    "desktop-user@local.slan",
		Password: "Desktop-2026!",
	})
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	device, err := dbDeviceService{state: state}.Register(auth.UserID, dto.RegisterDeviceRequest{
		Name:      "SLAN Client Windows",
		Platform:  "windows",
		MachineID: "client-machine-1",
		PublicKey: "device-key-1",
	})
	if err != nil {
		t.Fatalf("register device: %v", err)
	}

	user, err := state.pg.GetUserByID(ctx, auth.UserID)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	if user.ActiveNetworkID != "" {
		t.Fatalf("expected active network to stay empty, got %s", user.ActiveNetworkID)
	}

	attachments, err := state.pg.ListAttachmentsByDevice(ctx, device.DeviceID)
	if err != nil {
		t.Fatalf("list device attachments: %v", err)
	}
	if len(attachments) != 0 {
		t.Fatalf("expected no attachment before explicit network activation, got %+v", attachments)
	}

	if device.CurrentVirtualIP != "" {
		t.Fatalf("expected no current virtual ip before explicit network activation, got %+v", device)
	}
}

func TestCreateNetwork_BindsRequestedDevice(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "owner-device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}

	created, err := dbNetworkService{state: state}.Create("user-1", dto.CreateNetworkRequest{
		Name:         "primary",
		CIDR:         "10.0.0.0/16",
		BindDeviceID: "dev-1",
	})
	if err != nil {
		t.Fatalf("create network with binding: %v", err)
	}

	members, err := state.pg.ListMembersByNetwork(ctx, created.NetworkID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 1 || members[0].Role != "owner" || members[0].DeviceID != "dev-1" {
		t.Fatalf("expected bound owner member, got %+v", members)
	}

	attachments, err := state.pg.ListAttachmentsByDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("list attachments: %v", err)
	}
	if len(attachments) != 0 {
		t.Fatalf("expected no attachment until network activation, got %+v", attachments)
	}
}

func TestCreateNetwork_ActivatesFirstNetworkWhenNoneActive(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	created, err := dbNetworkService{state: state}.Create("user-1", dto.CreateNetworkRequest{
		Name: "primary",
		CIDR: "10.0.0.0/16",
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}

	user, err := state.pg.GetUserByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if user.ActiveNetworkID != created.NetworkID {
		t.Fatalf("expected active network %s, got %s", created.NetworkID, user.ActiveNetworkID)
	}
}

func TestCreateNetwork_PreservesExistingActiveNetwork(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:          "user-1",
		Email:           "owner@local.slan",
		PasswordHash:    "hash",
		ActiveNetworkID: "net-joined",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	createNetworkFixture(t, state, "owner-2", "net-joined", "subnet-joined", "10.9.0.0/16")

	created, err := dbNetworkService{state: state}.Create("user-1", dto.CreateNetworkRequest{
		Name: "primary",
		CIDR: "10.0.0.0/16",
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	if created.NetworkID == "net-joined" {
		t.Fatalf("expected newly created network id")
	}

	user, err := state.pg.GetUserByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if user.ActiveNetworkID != "net-joined" {
		t.Fatalf("expected active network to stay net-joined, got %s", user.ActiveNetworkID)
	}
}

func TestNewSubnet_RejectsPartialDhcpRange(t *testing.T) {
	_, err := newSubnet(
		"net-1",
		"subnet-1",
		"default",
		"10.0.0.0/24",
		"",
		"10.0.0.20",
		"",
		false,
	)
	if !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "allocationStartIp and allocationEndIp must be provided together") {
		t.Fatalf("expected paired dhcp range error, got %v", err)
	}
}

func TestNewSubnet_RejectsDefaultSubnetWithoutOwnerIP(t *testing.T) {
	_, err := newSubnet(
		"net-1",
		"subnet-1",
		"default",
		"10.0.0.0/24",
		"10.0.0.1",
		"10.0.0.3",
		"10.0.0.50",
		true,
	)
	if !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "default subnet DHCP range must include owner ip 10.0.0.2") {
		t.Fatalf("expected owner dhcp pairing error, got %v", err)
	}
}

func TestJoinNetwork_SwitchesDeviceToTargetNetwork(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "mac",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "10.0.0.0/16")
	createNetworkFixture(t, state, "user-1", "net-2", "subnet-2", "10.1.0.0/16")
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-1",
		NetworkID: "net-1",
		DeviceID:  "dev-1",
		Role:      "member",
		Status:    "active",
	}); err != nil {
		t.Fatalf("create old member: %v", err)
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-1",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-1",
		VirtualIP:    "10.0.0.10",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create old attachment: %v", err)
	}
	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "cs-1",
		UserID:           "user-1",
		DeviceID:         "dev-1",
		NodeID:           "node-1",
		NetworkID:        "net-1",
		SessionToken:     "token-1",
		ConnectedAt:      1,
		LastSeenAt:       1,
	}); err != nil {
		t.Fatalf("create old control session: %v", err)
	}

	result, err := dbNetworkService{state: state}.Join("user-1", "net-2", dto.JoinNetworkRequest{
		DeviceID: "dev-1",
	})
	if err != nil {
		t.Fatalf("join target network: %v", err)
	}
	if result.Member.NetworkID != "net-2" || result.Attachment.NetworkID != "" {
		t.Fatalf("expected join result to point at net-2, got member=%s attachment=%s", result.Member.NetworkID, result.Attachment.NetworkID)
	}

	members, err := state.pg.ListMembersByNetwork(ctx, "net-1")
	if err != nil {
		t.Fatalf("list old network members: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("expected old network members to be cleared, got %d", len(members))
	}

	attachments, err := state.pg.ListAttachmentsByDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("list device attachments: %v", err)
	}
	if len(attachments) != 0 {
		t.Fatalf("expected no attachment before activation, got %+v", attachments)
	}

	if _, err := state.pg.GetLatestControlSessionByNode(ctx, "node-1", "net-1"); !repo.IsNotFound(err) {
		t.Fatalf("expected old control session to be removed, got %v", err)
	}

	visible, err := dbNetworkService{state: state}.List("user-1")
	if err != nil {
		t.Fatalf("list visible networks: %v", err)
	}
	if len(visible) != 1 || visible[0].NetworkID != "net-2" {
		t.Fatalf("expected only net-2 to remain active, got %+v", visible)
	}
}

func TestUpdateNetwork_ReassignsVirtualIPsForAllMembers(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "owner-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "member@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create second user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "owner-1",
		MachineID: "machine-1",
		Name:      "owner-device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create owner device: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "member-device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create member device: %v", err)
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-1",
		NetworkID: "net-1",
		DeviceID:  "dev-1",
		Role:      "owner",
		Status:    "active",
	}); err != nil {
		t.Fatalf("create owner member: %v", err)
	}
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-2",
		NetworkID: "net-1",
		DeviceID:  "dev-2",
		Role:      "member",
		Status:    "active",
	}); err != nil {
		t.Fatalf("create member: %v", err)
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-2",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-2",
		VirtualIP:    "10.0.0.20",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create member attachment: %v", err)
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-1",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-1",
		VirtualIP:    "10.0.0.10",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create owner attachment: %v", err)
	}

	updated, err := dbNetworkService{state: state}.Update("owner-1", "net-1", dto.UpdateNetworkRequest{
		Name:              "office",
		Description:       "updated",
		CIDR:              "10.9.0.0/24",
		AllocationStartIP: "10.9.0.2",
		AllocationEndIP:   "10.9.0.20",
	})
	if err != nil {
		t.Fatalf("update network: %v", err)
	}
	if updated.DefaultSubnetCIDR != "10.9.0.0/24" {
		t.Fatalf("expected updated cidr, got %+v", updated)
	}

	subnet, err := state.pg.GetSubnetByID(ctx, "subnet-1")
	if err != nil {
		t.Fatalf("reload subnet: %v", err)
	}
	if subnet.CIDR != "10.9.0.0/24" {
		t.Fatalf("expected subnet cidr updated, got %s", subnet.CIDR)
	}
	if subnet.AllocationStartIP != "10.9.0.2" || subnet.AllocationEndIP != "10.9.0.20" {
		t.Fatalf("expected subnet dhcp range updated, got %+v", subnet)
	}

	attachments, err := state.pg.ListAttachmentsBySubnet(ctx, "subnet-1")
	if err != nil {
		t.Fatalf("list attachments: %v", err)
	}
	if len(attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(attachments))
	}
	if attachments[0].AttachmentID != "att-1" || attachments[0].VirtualIP != "10.9.0.2" {
		t.Fatalf("expected att-1 reassigned to 10.9.0.2, got %+v", attachments[0])
	}
	if attachments[1].AttachmentID != "att-2" || attachments[1].VirtualIP != "10.9.0.3" {
		t.Fatalf("expected att-2 reassigned to 10.9.0.3, got %+v", attachments[1])
	}

	tokenStore := state.tokens.(*memoryTokenStore)
	types := make([]string, 0, len(tokenStore.controlSyncEvents))
	for _, event := range tokenStore.controlSyncEvents {
		types = append(types, event.Type)
	}
	wantTypes := []string{"device_ip_reassigned", "device_ip_reassigned", "network_restart_required"}
	if strings.Join(types, ",") != strings.Join(wantTypes, ",") {
		t.Fatalf("expected reassignment events %v, got %+v", wantTypes, tokenStore.controlSyncEvents)
	}
	if first := tokenStore.controlSyncEvents[0].DeviceIP; first == nil || first.DeviceID != "dev-1" || first.VirtualIP != "10.9.0.2" {
		t.Fatalf("expected owner device reassigned to 10.9.0.2, got %+v", tokenStore.controlSyncEvents[0])
	}
	if second := tokenStore.controlSyncEvents[1].DeviceIP; second == nil || second.DeviceID != "dev-2" || second.VirtualIP != "10.9.0.3" {
		t.Fatalf("expected member device reassigned to 10.9.0.3, got %+v", tokenStore.controlSyncEvents[1])
	}
}

func TestHome_ReturnsOwnedAndActiveNetworks(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:          "user-1",
		Email:           "user@local.slan",
		PasswordHash:    "hash",
		ActiveNetworkID: "net-2",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "10.0.0.0/16")
	createNetworkFixture(t, state, "owner-2", "net-2", "subnet-2", "10.1.0.0/16")

	home, err := dbNetworkService{state: state}.Home("user-1")
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	if !home.HasNetwork {
		t.Fatalf("expected hasNetwork")
	}
	if home.OwnedNetwork == nil || home.OwnedNetwork.NetworkID != "net-1" {
		t.Fatalf("expected owned net-1, got %+v", home.OwnedNetwork)
	}
	if home.ActiveNetwork == nil || home.ActiveNetwork.NetworkID != "net-2" {
		t.Fatalf("expected active net-2, got %+v", home.ActiveNetwork)
	}
}

func TestJoinByKey_CreatesPendingMember(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "owner-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "member@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "member-device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create member device: %v", err)
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")
	setNetworkJoinKey(t, state, "net-1", "0123456789abcdef0123456789abcdef")

	result, err := dbNetworkService{state: state}.JoinByKey("user-2", dto.JoinNetworkByKeyRequest{
		JoinKey:  "0123456789abcdef0123456789abcdef",
		DeviceID: "dev-2",
	})
	if err != nil {
		t.Fatalf("join by key: %v", err)
	}
	if result.Member.Status != "pending" {
		t.Fatalf("expected pending join request, got %+v", result.Member)
	}
	if result.Attachment.VirtualIP != "" {
		t.Fatalf("expected no ip before activation, got %+v", result.Attachment)
	}
}

func TestInviteMember_CreatesPendingMemberForRegisteredUser(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "owner-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "member@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "member-device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create member device: %v", err)
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")

	member, err := dbNetworkService{state: state}.InviteMember("owner-1", "net-1", dto.InviteNetworkMemberRequest{
		Email: "member@local.slan",
	})
	if err != nil {
		t.Fatalf("invite member: %v", err)
	}
	if member.NetworkID != "net-1" || member.DeviceID != "dev-2" || member.Status != "pending" {
		t.Fatalf("expected pending member for dev-2 in net-1, got %+v", member)
	}
}

func TestJoinByKey_ConsumesKeyAfterOneUse(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "owner-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "member-1@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create member user 1: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-3",
		Email:        "member-2@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create member user 2: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "member-device-1",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create member device 1: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-3",
		UserID:    "user-3",
		MachineID: "machine-3",
		Name:      "member-device-2",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create member device 2: %v", err)
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")

	networkService := dbNetworkService{state: state}
	detail, err := networkService.UpdateJoinKey("owner-1", "net-1", dto.UpdateNetworkJoinKeyRequest{})
	if err != nil {
		t.Fatalf("set join key: %v", err)
	}
	if len(detail.JoinKey) != 32 {
		t.Fatalf("expected generated 32-char join key, got %q", detail.JoinKey)
	}

	result, err := networkService.JoinByKey("user-2", dto.JoinNetworkByKeyRequest{
		JoinKey:  detail.JoinKey,
		DeviceID: "dev-2",
	})
	if err != nil {
		t.Fatalf("join by key first use: %v", err)
	}
	if result.Member.NetworkID != "net-1" {
		t.Fatalf("expected joined net-1, got %+v", result)
	}
	if result.Member.Status != "pending" {
		t.Fatalf("expected join by key to create pending request, got %+v", result.Member)
	}

	record, err := state.pg.GetNetworkByID(ctx, "net-1")
	if err != nil {
		t.Fatalf("reload network: %v", err)
	}
	if record.JoinKey != "" {
		t.Fatalf("expected join key to be consumed, got %q", record.JoinKey)
	}

	if _, err := networkService.JoinByKey("user-3", dto.JoinNetworkByKeyRequest{
		JoinKey:  detail.JoinKey,
		DeviceID: "dev-3",
	}); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected second use to fail with not found, got %v", err)
	}
}

func TestPendingJoinRequiresOwnerApprovalBeforeActivation(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "owner-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "member@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "member-device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create member device: %v", err)
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")
	setNetworkJoinKey(t, state, "net-1", "0123456789abcdef0123456789abcdef")

	networkService := dbNetworkService{state: state}
	joined, err := networkService.JoinByKey("user-2", dto.JoinNetworkByKeyRequest{
		JoinKey:  "0123456789abcdef0123456789abcdef",
		DeviceID: "dev-2",
	})
	if err != nil {
		t.Fatalf("join by key: %v", err)
	}
	if joined.Member.Status != "pending" {
		t.Fatalf("expected pending member, got %+v", joined.Member)
	}
	if _, err := networkService.Activate("user-2", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-2"}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected pending activation to be forbidden, got %v", err)
	}
	pendingVisible, err := networkService.List("user-2")
	if err != nil {
		t.Fatalf("list pending networks: %v", err)
	}
	if len(pendingVisible) != 0 {
		t.Fatalf("expected pending network to stay hidden from member visibility, got %+v", pendingVisible)
	}

	approved, err := networkService.UpdateMemberStatus("owner-1", "net-1", joined.Member.MemberID, dto.UpdateNetworkMemberStatusRequest{
		Status: "active",
	})
	if err != nil {
		t.Fatalf("approve member: %v", err)
	}
	if approved.Status != "active" {
		t.Fatalf("expected approved member, got %+v", approved)
	}
	tokenStore := state.tokens.(*memoryTokenStore)
	if len(tokenStore.controlSyncEvents) != 1 {
		t.Fatalf("expected approval control sync event, got %+v", tokenStore.controlSyncEvents)
	}
	event := tokenStore.controlSyncEvents[0]
	if event.Type != "active_network_enabled" || event.TargetUserID != "user-2" || event.NetworkID != "net-1" {
		t.Fatalf("expected active network enabled event for applicant, got %+v", event)
	}
	if event.ActiveNetwork == nil || event.ActiveNetwork.UserID != "user-2" || event.ActiveNetwork.NetworkID != "net-1" {
		t.Fatalf("expected active network payload for applicant, got %+v", event.ActiveNetwork)
	}
	activeVisible, err := networkService.List("user-2")
	if err != nil {
		t.Fatalf("list active networks: %v", err)
	}
	if len(activeVisible) != 1 || activeVisible[0].NetworkID != "net-1" {
		t.Fatalf("expected approved network to become visible, got %+v", activeVisible)
	}
	activated, err := networkService.Activate("user-2", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-2"})
	if err != nil {
		t.Fatalf("activate after approval: %v", err)
	}
	if activated.Attachment.VirtualIP == "" {
		t.Fatalf("expected virtual ip after approval, got %+v", activated)
	}
}

func TestRejectedJoinCanBeRequestedAgain(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "owner-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "member@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "member-device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create member device: %v", err)
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")
	setNetworkJoinKey(t, state, "net-1", "0123456789abcdef0123456789abcdef")

	networkService := dbNetworkService{state: state}
	first, err := networkService.JoinByKey("user-2", dto.JoinNetworkByKeyRequest{
		JoinKey:  "0123456789abcdef0123456789abcdef",
		DeviceID: "dev-2",
	})
	if err != nil {
		t.Fatalf("join by key: %v", err)
	}
	if _, err := networkService.UpdateMemberStatus("owner-1", "net-1", first.Member.MemberID, dto.UpdateNetworkMemberStatusRequest{
		Status: "rejected",
	}); err != nil {
		t.Fatalf("reject member: %v", err)
	}
	setNetworkJoinKey(t, state, "net-1", "0123456789abcdef0123456789abcdef")
	second, err := networkService.JoinByKey("user-2", dto.JoinNetworkByKeyRequest{
		JoinKey:  "0123456789abcdef0123456789abcdef",
		DeviceID: "dev-2",
	})
	if err != nil {
		t.Fatalf("re-request join by key: %v", err)
	}
	if second.Member.MemberID != first.Member.MemberID || second.Member.Status != "pending" {
		t.Fatalf("expected rejected membership to reopen as pending, got first=%+v second=%+v", first.Member, second.Member)
	}
}

func TestRejectActiveMemberCleansRuntimeState(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now().Unix()

	for _, user := range []repo.User{
		{UserID: "owner-1", Email: "owner@local.slan", PasswordHash: "hash", ActiveNetworkID: "net-1"},
		{UserID: "user-2", Email: "member@local.slan", PasswordHash: "hash", ActiveNetworkID: "net-1"},
	} {
		if err := state.pg.CreateUser(ctx, user); err != nil {
			t.Fatalf("create user %s: %v", user.UserID, err)
		}
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "member-device",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create member device: %v", err)
	}
	if err := state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        "node-2",
		UserID:        "user-2",
		DeviceID:      "dev-2",
		NodePublicKey: "node-public-key",
		Capabilities:  []string{"desktop"},
	}); err != nil {
		t.Fatalf("create member node: %v", err)
	}
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-2",
		NetworkID: "net-1",
		DeviceID:  "dev-2",
		Role:      "member",
		Status:    "active",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create active member: %v", err)
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-2",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-2",
		VirtualIP:    "10.0.0.3",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create member attachment: %v", err)
	}
	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-2",
		UserID:           "user-2",
		DeviceID:         "dev-2",
		NodeID:           "node-2",
		NetworkID:        "net-1",
		SessionToken:     "token-2",
		ConnectedAt:      now,
		LastSeenAt:       now,
	}); err != nil {
		t.Fatalf("create control session: %v", err)
	}

	rejected, err := dbNetworkService{state: state}.UpdateMemberStatus("owner-1", "net-1", "member-2", dto.UpdateNetworkMemberStatusRequest{
		Status: "rejected",
	})
	if err != nil {
		t.Fatalf("reject member: %v", err)
	}
	if rejected.Status != "rejected" {
		t.Fatalf("expected rejected member, got %+v", rejected)
	}
	attachments, err := state.pg.ListAttachmentsByDevice(ctx, "dev-2")
	if err != nil {
		t.Fatalf("list attachments: %v", err)
	}
	if len(attachments) != 0 {
		t.Fatalf("expected rejected member attachments to be deleted, got %+v", attachments)
	}
	if _, err := state.pg.GetLatestControlSessionByNode(ctx, "node-2", "net-1"); !repo.IsNotFound(err) {
		t.Fatalf("expected rejected member control session to be deleted, got %v", err)
	}
	user, err := state.pg.GetUserByID(ctx, "user-2")
	if err != nil {
		t.Fatalf("load member user: %v", err)
	}
	if user.ActiveNetworkID != "" {
		t.Fatalf("expected rejected member active network to be cleared, got %s", user.ActiveNetworkID)
	}
	tokenStore := state.tokens.(*memoryTokenStore)
	if len(tokenStore.controlSyncEvents) != 1 || tokenStore.controlSyncEvents[0].Type != "peer_remove" {
		t.Fatalf("expected peer_remove control sync event, got %+v", tokenStore.controlSyncEvents)
	}
}

func TestSwitch_ActivatesOwnedNetwork(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "owner-2",
		Email:        "owner-2@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create owner-2: %v", err)
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "10.0.0.0/16")
	createNetworkFixture(t, state, "owner-2", "net-2", "subnet-2", "10.1.0.0/16")
	setNetworkJoinKey(t, state, "net-2", "fedcba9876543210fedcba9876543210")

	service := dbNetworkService{state: state}
	joined, err := service.JoinByKey("user-1", dto.JoinNetworkByKeyRequest{
		JoinKey:  "fedcba9876543210fedcba9876543210",
		DeviceID: "dev-1",
	})
	if err != nil {
		t.Fatalf("join foreign network by key: %v", err)
	}
	if joined.Member.Status != "pending" {
		t.Fatalf("expected pending foreign network request, got %+v", joined.Member)
	}
	result, err := service.Switch("user-1", "net-1", dto.SwitchNetworkRequest{DeviceID: "dev-1"})
	if err != nil {
		t.Fatalf("switch to own network: %v", err)
	}
	if result.Member.NetworkID != "net-1" || result.Attachment.NetworkID != "" {
		t.Fatalf("expected switched to net-1, got %+v", result)
	}

	visible, err := dbNetworkService{state: state}.List("user-1")
	if err != nil {
		t.Fatalf("list after switch: %v", err)
	}
	if len(visible) != 1 || visible[0].NetworkID != "net-1" {
		t.Fatalf("expected active net-1, got %+v", visible)
	}
}

func TestActivateAndDeactivate_AllocatesIpOnlyWhileEnabled(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "10.0.0.0/16")

	service := dbNetworkService{state: state}
	if _, err := service.Join("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-1"}); err != nil {
		t.Fatalf("join network: %v", err)
	}

	before, err := state.pg.ListAttachmentsByDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("list attachments before activation: %v", err)
	}
	if len(before) != 0 {
		t.Fatalf("expected no attachments before activation, got %+v", before)
	}

	activated, err := service.Activate("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-1"})
	if err != nil {
		t.Fatalf("activate network: %v", err)
	}
	if activated.Attachment.NetworkID != "net-1" || activated.Attachment.VirtualIP == "" {
		t.Fatalf("expected attachment with allocated ip after activation, got %+v", activated.Attachment)
	}
	if err := state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        "node-1",
		UserID:        "user-1",
		DeviceID:      "dev-1",
		NodePublicKey: "node-public-key",
		Capabilities:  []string{"desktop"},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}
	now := time.Now().Unix()
	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-1",
		UserID:           "user-1",
		DeviceID:         "dev-1",
		NodeID:           "node-1",
		NetworkID:        "net-1",
		SessionToken:     "token-1",
		ConnectedAt:      now,
		LastSeenAt:       now,
	}); err != nil {
		t.Fatalf("create control session: %v", err)
	}
	if err := state.tokens.StoreControlSessionToken(ctx, "token-1", "user-1", time.Hour); err != nil {
		t.Fatalf("store control session token: %v", err)
	}

	afterActivate, err := state.pg.ListAttachmentsByDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("list attachments after activation: %v", err)
	}
	if len(afterActivate) != 1 || afterActivate[0].VirtualIP == "" {
		t.Fatalf("expected one active attachment after activation, got %+v", afterActivate)
	}

	if err := service.Deactivate("user-1", "net-1", dto.DeactivateNetworkRequest{DeviceID: "dev-1"}); err != nil {
		t.Fatalf("deactivate network: %v", err)
	}

	afterDeactivate, err := state.pg.ListAttachmentsByDevice(ctx, "dev-1")
	if err != nil {
		t.Fatalf("list attachments after deactivation: %v", err)
	}
	if len(afterDeactivate) != 0 {
		t.Fatalf("expected no attachments after deactivation, got %+v", afterDeactivate)
	}
	user, err := state.pg.GetUserByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("reload user after deactivation: %v", err)
	}
	if user.ActiveNetworkID != "" {
		t.Fatalf("expected active network pointer to be cleared after deactivation, got %s", user.ActiveNetworkID)
	}
	if _, err := state.pg.GetLatestControlSessionByNode(ctx, "node-1", "net-1"); !repo.IsNotFound(err) {
		t.Fatalf("expected control session to be deleted after deactivation, got %v", err)
	}
	tokenStore := state.tokens.(*memoryTokenStore)
	if _, err := tokenStore.AuthenticateControlSessionToken(ctx, "token-1"); err == nil {
		t.Fatal("expected control session token to be revoked after deactivation")
	}
	if len(tokenStore.controlSyncEvents) != 1 || tokenStore.controlSyncEvents[0].Type != "peer_remove" {
		t.Fatalf("expected peer_remove control sync event, got %+v", tokenStore.controlSyncEvents)
	}
}

func TestActivate_RejectsSixthFreeActiveDevice(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	for i := 1; i <= 6; i++ {
		deviceID := fmt.Sprintf("dev-%d", i)
		if err := state.pg.InsertDevice(ctx, repo.Device{
			DeviceID:  deviceID,
			UserID:    "user-1",
			MachineID: fmt.Sprintf("machine-%d", i),
			Name:      deviceID,
			Platform:  "windows",
			Status:    "online",
		}); err != nil {
			t.Fatalf("create device %s: %v", deviceID, err)
		}
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "10.0.0.0/24")

	networkService := dbNetworkService{state: state}
	for _, deviceID := range []string{"dev-1", "dev-2", "dev-3", "dev-4", "dev-5"} {
		if _, err := networkService.Join("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: deviceID}); err != nil {
			t.Fatalf("join %s: %v", deviceID, err)
		}
		if _, err := networkService.Activate("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: deviceID}); err != nil {
			t.Fatalf("activate %s: %v", deviceID, err)
		}
	}
	if _, err := networkService.Join("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-6"}); err != nil {
		t.Fatalf("join dev-6: %v", err)
	}
	if _, err := networkService.Activate("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-6"}); !errors.Is(err, service.ErrDeviceLimitExceeded) {
		t.Fatalf("expected fixed free limit to reject sixth active device, got %v", err)
	}
}

func TestActivate_UsesGlobalAndUserDeviceLimitConfig(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	for i := 1; i <= 3; i++ {
		deviceID := fmt.Sprintf("dev-%d", i)
		if err := state.pg.InsertDevice(ctx, repo.Device{
			DeviceID:  deviceID,
			UserID:    "user-1",
			MachineID: fmt.Sprintf("machine-%d", i),
			Name:      deviceID,
			Platform:  "windows",
			Status:    "online",
		}); err != nil {
			t.Fatalf("create device %s: %v", deviceID, err)
		}
	}
	if err := state.pg.UpsertPlanConfig(ctx, repo.PlanConfig{
		ConfigID:         globalPlanConfigID,
		MaxActiveDevices: 2,
		RelayIngressKbps: 100,
		RelayEgressKbps:  200,
		UDPIngressKbps:   300,
		UDPEgressKbps:    400,
	}); err != nil {
		t.Fatalf("set global plan: %v", err)
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "10.0.0.0/24")

	networkService := dbNetworkService{state: state}
	for _, deviceID := range []string{"dev-1", "dev-2"} {
		if _, err := networkService.Join("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: deviceID}); err != nil {
			t.Fatalf("join %s: %v", deviceID, err)
		}
		if _, err := networkService.Activate("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: deviceID}); err != nil {
			t.Fatalf("activate %s: %v", deviceID, err)
		}
	}
	if _, err := networkService.Join("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-3"}); err != nil {
		t.Fatalf("join dev-3: %v", err)
	}
	if _, err := networkService.Activate("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-3"}); !errors.Is(err, service.ErrDeviceLimitExceeded) {
		t.Fatalf("expected global limit to reject third active device, got %v", err)
	}
	if err := state.pg.UpsertUserPlanOverride(ctx, repo.UserPlanOverride{
		UserID:           "user-1",
		MaxActiveDevices: 3,
		RelayIngressKbps: 1000,
		RelayEgressKbps:  1001,
		UDPIngressKbps:   1002,
		UDPEgressKbps:    1003,
	}); err != nil {
		t.Fatalf("set user plan override: %v", err)
	}
	if _, err := networkService.Activate("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-3"}); err != nil {
		t.Fatalf("expected user override to allow third active device, got %v", err)
	}
	plan := state.planStatusForUser(ctx, "user-1")
	if plan.MaxActiveDevices != 3 ||
		plan.RelayIngressKbps != 1000 ||
		plan.RelayEgressKbps != 1001 ||
		plan.UDPIngressKbps != 1002 ||
		plan.UDPEgressKbps != 1003 ||
		plan.P2PUnlimited {
		t.Fatalf("expected user override plan to win over global config, got %+v", plan)
	}
}

func TestBootstrap_IncludesAttachmentsAfterActivation(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "10.0.0.0/16")
	if err := state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        "node-1",
		UserID:        "user-1",
		DeviceID:      "dev-1",
		NodePublicKey: "pubkey-1",
		Capabilities:  []string{},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	service := dbNetworkService{state: state}
	if _, err := service.Join("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-1"}); err != nil {
		t.Fatalf("join network: %v", err)
	}
	if _, err := service.Activate("user-1", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-1"}); err != nil {
		t.Fatalf("activate network: %v", err)
	}

	bootstrap, err := dbBootstrapService{state: state}.Bootstrap("user-1", dto.BootstrapRequest{
		NodeID:    "node-1",
		NetworkID: "net-1",
	})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if len(bootstrap.Device.Attachments) != 1 {
		t.Fatalf("expected one attachment in bootstrap, got %+v", bootstrap.Device.Attachments)
	}
	if bootstrap.Device.Attachments[0].VirtualIP == "" {
		t.Fatalf("expected bootstrap attachment ip, got %+v", bootstrap.Device.Attachments[0])
	}
}

func TestUpdateAttachmentIP_UpdatesLease(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "owner-1",
		Email:        "owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "owner-1",
		MachineID: "machine-1",
		Name:      "owner-device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "member@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "member-device",
		Platform:  "macos",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create member device: %v", err)
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-1",
		NetworkID: "net-1",
		DeviceID:  "dev-1",
		Role:      "owner",
		Status:    "active",
	}); err != nil {
		t.Fatalf("create owner member: %v", err)
	}
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-2",
		NetworkID: "net-1",
		DeviceID:  "dev-2",
		Role:      "member",
		Status:    "active",
	}); err != nil {
		t.Fatalf("create member relation: %v", err)
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-1",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-1",
		VirtualIP:    "10.0.0.2",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create owner attachment: %v", err)
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-2",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-2",
		VirtualIP:    "10.0.0.3",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create member attachment: %v", err)
	}

	updated, err := dbNetworkService{state: state}.UpdateAttachmentIP("owner-1", "net-1", "att-2", dto.UpdateAttachmentIPRequest{
		VirtualIP: "10.0.0.9",
	})
	if err != nil {
		t.Fatalf("update attachment ip: %v", err)
	}
	if updated.VirtualIP != "10.0.0.9" {
		t.Fatalf("expected updated virtual ip, got %+v", updated)
	}
	ownerUpdated, err := dbNetworkService{state: state}.UpdateAttachmentIP("owner-1", "net-1", "att-1", dto.UpdateAttachmentIPRequest{
		VirtualIP: "10.0.0.4",
	})
	if err != nil {
		t.Fatalf("owner attachment ip should be editable: %v", err)
	}
	if ownerUpdated.VirtualIP != "10.0.0.4" {
		t.Fatalf("expected owner virtual ip to update, got %+v", ownerUpdated)
	}
}

func TestUpdateAttachmentStatus_DisablesMemberAndNotifiesDevice(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	for _, user := range []repo.User{
		{UserID: "owner-1", Email: "owner@local.slan", PasswordHash: "hash", ActiveNetworkID: "net-1"},
		{UserID: "user-2", Email: "member@local.slan", PasswordHash: "hash", ActiveNetworkID: "net-1"},
	} {
		if err := state.pg.CreateUser(ctx, user); err != nil {
			t.Fatalf("create user %s: %v", user.UserID, err)
		}
	}
	for _, device := range []repo.Device{
		{DeviceID: "dev-1", UserID: "owner-1", MachineID: "machine-1", Name: "owner-device", Platform: "macos", Status: "online"},
		{DeviceID: "dev-2", UserID: "user-2", MachineID: "machine-2", Name: "member-device", Platform: "windows", Status: "online"},
	} {
		if err := state.pg.InsertDevice(ctx, device); err != nil {
			t.Fatalf("create device %s: %v", device.DeviceID, err)
		}
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")
	for _, member := range []dto.NetworkMember{
		{MemberID: "member-1", NetworkID: "net-1", DeviceID: "dev-1", Role: "owner", Status: "active"},
		{MemberID: "member-2", NetworkID: "net-1", DeviceID: "dev-2", Role: "member", Status: "active"},
	} {
		if err := state.pg.CreateMember(ctx, member); err != nil {
			t.Fatalf("create member %s: %v", member.MemberID, err)
		}
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-2",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-2",
		VirtualIP:    "10.0.0.3",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create member attachment: %v", err)
	}
	if err := state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        "node-2",
		UserID:        "user-2",
		DeviceID:      "dev-2",
		NodePublicKey: "node-key-2",
		Capabilities:  []string{"desktop"},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-2",
		UserID:           "user-2",
		DeviceID:         "dev-2",
		NodeID:           "node-2",
		NetworkID:        "net-1",
		SessionToken:     "token-2",
		ConnectedAt:      time.Now().Unix(),
		LastSeenAt:       time.Now().Unix(),
	}); err != nil {
		t.Fatalf("create control session: %v", err)
	}
	if err := state.tokens.StoreControlSessionToken(ctx, "token-2", "user-2", time.Hour); err != nil {
		t.Fatalf("store control token: %v", err)
	}

	updated, err := (dbNetworkService{state: state}).UpdateAttachmentStatus("owner-1", "net-1", "att-2", dto.UpdateAttachmentStatusRequest{Status: "disabled"})
	if err != nil {
		t.Fatalf("disable attachment: %v", err)
	}
	if updated.Status != "disabled" || updated.VirtualIP != "" {
		t.Fatalf("expected disabled assignment without virtual ip, got %+v", updated)
	}
	if member, err := state.pg.GetMemberByID(ctx, "member-2"); err != nil || member.Status != "active" {
		t.Fatalf("expected member to remain active for later re-enable, got member=%+v err=%v", member, err)
	}
	attachments, err := state.pg.ListAttachmentsByDevice(ctx, "dev-2")
	if err != nil {
		t.Fatalf("list attachments: %v", err)
	}
	if len(attachments) != 1 || attachments[0].Status != "disabled" || attachments[0].VirtualIP != "" {
		t.Fatalf("expected disabled attachment to be kept, got %+v", attachments)
	}
	if _, err := (dbNetworkService{state: state}).Activate("user-2", "net-1", dto.JoinNetworkRequest{DeviceID: "dev-2"}); !errors.Is(err, service.ErrForbidden) || !strings.Contains(err.Error(), "device unavailable") {
		t.Fatalf("expected disabled device activation to be blocked, got %v", err)
	}
	if _, err := state.pg.GetLatestControlSessionByNode(ctx, "node-2", "net-1"); !repo.IsNotFound(err) {
		t.Fatalf("expected control session to be deleted, got %v", err)
	}
	if _, err := state.tokens.AuthenticateControlSessionToken(ctx, "token-2"); err == nil {
		t.Fatal("expected control session token to be revoked")
	}
	memberUser, err := state.pg.GetUserByID(ctx, "user-2")
	if err != nil {
		t.Fatalf("load member user: %v", err)
	}
	if memberUser.ActiveNetworkID != "" {
		t.Fatalf("expected member active network cleared, got %s", memberUser.ActiveNetworkID)
	}
	tokenStore := state.tokens.(*memoryTokenStore)
	types := make([]string, 0, len(tokenStore.controlSyncEvents))
	for _, event := range tokenStore.controlSyncEvents {
		types = append(types, event.Type)
	}
	want := []string{"device_ip_reassigned", "peer_remove", "network_restart_required"}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("expected events %v, got %+v", want, tokenStore.controlSyncEvents)
	}
	if event := tokenStore.controlSyncEvents[0]; event.DeviceIP == nil || event.DeviceIP.DeviceID != "dev-2" || event.DeviceIP.VirtualIP != "" {
		t.Fatalf("expected blank device ip reassignment, got %+v", event)
	}

	reenabled, err := (dbNetworkService{state: state}).UpdateAttachmentStatus("owner-1", "net-1", "att-2", dto.UpdateAttachmentStatusRequest{Status: "active"})
	if err != nil {
		t.Fatalf("re-enable attachment: %v", err)
	}
	if reenabled.Status != "active" || reenabled.VirtualIP == "" {
		t.Fatalf("expected re-enabled assignment with virtual ip, got %+v", reenabled)
	}
	memberUser, err = state.pg.GetUserByID(ctx, "user-2")
	if err != nil {
		t.Fatalf("reload member user: %v", err)
	}
	if memberUser.ActiveNetworkID != "net-1" {
		t.Fatalf("expected member active network restored, got %s", memberUser.ActiveNetworkID)
	}
}

func TestUpdateAttachmentStatus_AllowsOwnerDeviceDisable(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:          "owner-1",
		Email:           "owner@local.slan",
		PasswordHash:    "hash",
		ActiveNetworkID: "net-1",
	}); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "owner-1",
		MachineID: "machine-1",
		Name:      "owner-device",
		Platform:  "windows",
		Status:    "online",
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-1",
		NetworkID: "net-1",
		DeviceID:  "dev-1",
		Role:      "owner",
		Status:    "active",
	}); err != nil {
		t.Fatalf("create owner member: %v", err)
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-1",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-1",
		VirtualIP:    "10.0.0.2",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create owner attachment: %v", err)
	}
	if err := state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        "node-1",
		UserID:        "owner-1",
		DeviceID:      "dev-1",
		NodePublicKey: "node-key-1",
		Capabilities:  []string{"desktop"},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-1",
		UserID:           "owner-1",
		DeviceID:         "dev-1",
		NodeID:           "node-1",
		NetworkID:        "net-1",
		SessionToken:     "token-1",
		ConnectedAt:      time.Now().Unix(),
		LastSeenAt:       time.Now().Unix(),
	}); err != nil {
		t.Fatalf("create control session: %v", err)
	}
	if err := state.tokens.StoreControlSessionToken(ctx, "token-1", "owner-1", time.Hour); err != nil {
		t.Fatalf("store control token: %v", err)
	}

	updated, err := (dbNetworkService{state: state}).UpdateAttachmentStatus("owner-1", "net-1", "att-1", dto.UpdateAttachmentStatusRequest{Status: "disabled"})
	if err != nil {
		t.Fatalf("owner device should be disabled: %v", err)
	}
	if updated.Status != "disabled" || updated.VirtualIP != "" {
		t.Fatalf("expected owner assignment disabled, got %+v", updated)
	}
	if member, err := state.pg.GetMemberByID(ctx, "member-1"); err != nil || member.Status != "active" {
		t.Fatalf("expected owner member to remain active for later re-enable, got member=%+v err=%v", member, err)
	}
	user, err := state.pg.GetUserByID(ctx, "owner-1")
	if err != nil {
		t.Fatalf("load owner: %v", err)
	}
	if user.ActiveNetworkID != "" {
		t.Fatalf("expected owner active network cleared, got %s", user.ActiveNetworkID)
	}
	if _, err := state.pg.GetLatestControlSessionByNode(ctx, "node-1", "net-1"); !repo.IsNotFound(err) {
		t.Fatalf("expected owner control session to be deleted, got %v", err)
	}
	if _, err := state.tokens.AuthenticateControlSessionToken(ctx, "token-1"); err == nil {
		t.Fatal("expected owner control session token to be revoked")
	}
	tokenStore := state.tokens.(*memoryTokenStore)
	if len(tokenStore.controlSyncEvents) < 1 || tokenStore.controlSyncEvents[0].DeviceIP == nil {
		t.Fatalf("expected device ip reassignment event, got %+v", tokenStore.controlSyncEvents)
	}
	if event := tokenStore.controlSyncEvents[0]; event.DeviceIP.DeviceID != "dev-1" || event.DeviceIP.VirtualIP != "" {
		t.Fatalf("expected owner device to receive blank virtual ip, got %+v", event)
	}
}

func TestUpdateAttachmentRemark_AllowsDeviceOwnerAlias(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	for _, user := range []repo.User{
		{UserID: "owner-1", Email: "owner@local.slan", PasswordHash: "hash"},
		{UserID: "user-2", Email: "member@local.slan", PasswordHash: "hash"},
		{UserID: "user-3", Email: "other@local.slan", PasswordHash: "hash"},
	} {
		if err := state.pg.CreateUser(ctx, user); err != nil {
			t.Fatalf("create user %s: %v", user.UserID, err)
		}
	}
	for _, device := range []repo.Device{
		{DeviceID: "dev-1", UserID: "owner-1", MachineID: "machine-1", Name: "owner-device", Platform: "macos", Status: "online"},
		{DeviceID: "dev-2", UserID: "user-2", MachineID: "machine-2", Name: "member-device", Platform: "windows", Status: "online"},
		{DeviceID: "dev-3", UserID: "user-3", MachineID: "machine-3", Name: "other-device", Platform: "linux", Status: "online"},
	} {
		if err := state.pg.InsertDevice(ctx, device); err != nil {
			t.Fatalf("create device %s: %v", device.DeviceID, err)
		}
	}
	createNetworkFixture(t, state, "owner-1", "net-1", "subnet-1", "10.0.0.0/16")
	for _, member := range []dto.NetworkMember{
		{MemberID: "member-1", NetworkID: "net-1", DeviceID: "dev-1", Role: "owner", Status: "active"},
		{MemberID: "member-2", NetworkID: "net-1", DeviceID: "dev-2", Role: "member", Status: "active"},
	} {
		if err := state.pg.CreateMember(ctx, member); err != nil {
			t.Fatalf("create member %s: %v", member.MemberID, err)
		}
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-2",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-2",
		VirtualIP:    "10.0.0.3",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create member attachment: %v", err)
	}

	updated, err := (dbNetworkService{state: state}).UpdateAttachmentRemark("user-2", "net-1", "att-2", dto.UpdateAttachmentRemarkRequest{
		Remark: "Thor laptop",
	})
	if err != nil {
		t.Fatalf("member should update own attachment remark: %v", err)
	}
	if updated.Remark != "Thor laptop" {
		t.Fatalf("expected updated remark, got %+v", updated)
	}
	if _, err := (dbNetworkService{state: state}).UpdateAttachmentRemark("user-3", "net-1", "att-2", dto.UpdateAttachmentRemarkRequest{
		Remark: "not mine",
	}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected other user to be forbidden, got %v", err)
	}
}

func newNetworkTestState(t *testing.T) *dbState {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: "sqlite",
		DSN:        dsn,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&repo.User{},
		&repo.Device{},
		&repo.Network{},
		&repo.Subnet{},
		&repo.NetworkMember{},
		&repo.SubnetAttachment{},
		&repo.Node{},
		&repo.NodeEndpoint{},
		&repo.NodeConnectionState{},
		&repo.NodePathHealth{},
		&repo.ControlSession{},
		&repo.DeviceNetworkState{},
		&repo.PlanConfig{},
		&repo.UserPlanOverride{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return &dbState{
		cfg:    configs.DefaultConfig(),
		pg:     repo.NewPostgresRepository(db),
		tokens: newMemoryTokenStore(),
	}
}

func createNetworkFixture(t *testing.T, state *dbState, ownerUserID, networkID, subnetID, cidr string) {
	t.Helper()

	ctx := context.Background()
	subnet, err := newSubnet(networkID, subnetID, "default", cidr, "", "", "", true)
	if err != nil {
		t.Fatalf("build subnet fixture: %v", err)
	}
	if err := state.pg.CreateNetworkWithDefaultSubnet(ctx, ownerUserID, dto.Network{
		NetworkID:         networkID,
		Name:              networkID,
		DefaultSubnetID:   subnet.SubnetID,
		DefaultSubnetCIDR: subnet.CIDR,
	}, subnet); err != nil {
		t.Fatalf("create network fixture: %v", err)
	}
}

func setNetworkJoinKey(t *testing.T, state *dbState, networkID, joinKey string) {
	t.Helper()
	if err := state.pg.UpdateNetworkJoinKey(context.Background(), networkID, joinKey); err != nil {
		t.Fatalf("set join key: %v", err)
	}
}
