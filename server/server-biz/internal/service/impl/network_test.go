package impl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
		Name:        "office",
		Description: "updated",
		CIDR:        "10.9.0.0/24",
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

func TestJoinByOwnerEmail_JoinsOwnedNetwork(t *testing.T) {
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

	result, err := dbNetworkService{state: state}.JoinByOwnerEmail("user-2", dto.JoinNetworkByOwnerEmailRequest{
		OwnerEmail: "owner@local.slan",
		DeviceID:   "dev-2",
	})
	if err != nil {
		t.Fatalf("join by owner email: %v", err)
	}
	if result.Network.NetworkID != "net-1" {
		t.Fatalf("expected joined net-1, got %+v", result)
	}
	if result.Attachment.VirtualIP != "" {
		t.Fatalf("expected no ip before activation, got %+v", result.Attachment)
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

	service := dbNetworkService{state: state}
	if _, err := service.JoinByOwnerEmail("user-1", dto.JoinNetworkByOwnerEmailRequest{
		OwnerEmail: "owner-2@local.slan",
		DeviceID:   "dev-1",
	}); err != nil {
		t.Fatalf("join foreign network: %v", err)
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
}

func newNetworkTestState(t *testing.T) *dbState {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return &dbState{
		pg: repo.NewPostgresRepository(db),
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
