package service

import (
	"context"
	"testing"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type inviteRevocationTestUsers struct {
	repository.UserRepository
	users map[string]model.User
}

func (s inviteRevocationTestUsers) GetUser(_ context.Context, userID string) (model.User, bool, error) {
	item, ok := s.users[userID]
	return item, ok, nil
}

type inviteRevocationTestNetworks struct {
	repository.NetworkRepository
	invite model.DeviceInvite
}

type inviteRevocationTestRelations struct {
	repository.DeviceRelationRepository
	networks *inviteRevocationTestNetworks
	relation model.DeviceUserRelation
}

func (s *inviteRevocationTestRelations) SaveDeviceInviteWithRelation(_ context.Context, invite model.DeviceInvite, relation model.DeviceUserRelation) error {
	s.networks.invite = invite
	s.relation = relation
	return nil
}

func (s *inviteRevocationTestRelations) RevokeDeviceInviteWithRelation(_ context.Context, invite model.DeviceInvite, sharedUserID, revokedBy string, revokedAt int64) error {
	s.networks.invite = invite
	s.relation.UserID = sharedUserID
	s.relation.Status = model.DeviceRelationStatusRevoked
	s.relation.RevokedBy = revokedBy
	s.relation.RevokedAt = revokedAt
	return nil
}

func (s *inviteRevocationTestNetworks) GetDeviceInvite(_ context.Context, inviteID string) (model.DeviceInvite, bool, error) {
	return s.invite, inviteID == s.invite.InviteID, nil
}

func (s *inviteRevocationTestNetworks) GetDeviceInviteByCode(_ context.Context, inviteCode string) (model.DeviceInvite, bool, error) {
	return s.invite, inviteCode == s.invite.InviteCode, nil
}

func (s *inviteRevocationTestNetworks) SaveDeviceInvite(_ context.Context, invite model.DeviceInvite) error {
	s.invite = invite
	return nil
}

func TestResolveAcceptedDeviceInviteDeviceID(t *testing.T) {
	tests := []struct {
		name    string
		invite  model.DeviceInvite
		input   AcceptDeviceInviteInput
		want    string
		wantErr error
	}{
		{
			name:   "prefers input device when invite device missing",
			invite: model.DeviceInvite{InviteID: "invite-1", InviteCode: "ABCD"},
			input:  AcceptDeviceInviteInput{InviteCode: "ABCD", DeviceID: "device-1"},
			want:   "device-1",
		},
		{
			name:   "uses invite device when input missing",
			invite: model.DeviceInvite{InviteID: "invite-1", InviteCode: "ABCD", DeviceID: "device-2"},
			input:  AcceptDeviceInviteInput{InviteCode: "ABCD"},
			want:   "device-2",
		},
		{
			name:    "rejects mismatched device ids",
			invite:  model.DeviceInvite{InviteID: "invite-1", InviteCode: "ABCD", DeviceID: "device-2"},
			input:   AcceptDeviceInviteInput{InviteCode: "ABCD", DeviceID: "device-1"},
			wantErr: ErrConflict,
		},
		{
			name:    "rejects empty device id",
			invite:  model.DeviceInvite{InviteID: "invite-1", InviteCode: "ABCD"},
			input:   AcceptDeviceInviteInput{InviteCode: "ABCD"},
			wantErr: ErrInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAcceptedDeviceInviteDeviceID(tt.invite, tt.input)
			if err != tt.wantErr {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAcceptDeviceInviteCreatesSharedRelation(t *testing.T) {
	networks := &inviteRevocationTestNetworks{invite: model.DeviceInvite{
		InviteID: "invite-1", InviteCode: "ABCD", InviterUserID: "inviter",
		UserID: "inviter", Status: "pending", ExpiresAt: 4102444800,
	}}
	relations := &inviteRevocationTestRelations{networks: networks}
	service := NetworkInviteService{
		Users: inviteRevocationTestUsers{users: map[string]model.User{
			"inviter": {UserID: "inviter", Email: "inviter@example.test"},
			"owner":   {UserID: "owner", Email: "owner@example.test"},
		}},
		Devices: deviceVisibilityTestDevices{items: map[string]model.Device{
			"device-1": {DeviceID: "device-1", OwnerID: "owner"},
		}},
		Networks:  networks,
		Relations: relations,
	}

	view, err := service.AcceptDeviceInvite(context.Background(), AcceptDeviceInviteInput{
		InviteCode: "ABCD", ActorUserID: "owner", UserID: "owner", DeviceID: "device-1",
	})
	if err != nil {
		t.Fatalf("accept invite: %v", err)
	}
	if view.Status != "accepted" || networks.invite.Status != "accepted" {
		t.Fatalf("expected accepted invite, view=%q stored=%q", view.Status, networks.invite.Status)
	}
	if relations.relation.DeviceID != "device-1" || relations.relation.UserID != "inviter" ||
		relations.relation.Role != model.DeviceRelationRoleShared || relations.relation.Status != model.DeviceRelationStatusActive {
		t.Fatalf("unexpected shared relation: %#v", relations.relation)
	}
}

func TestRevokeDeviceInviteAllowsInviterAndDeviceOwner(t *testing.T) {
	tests := []struct {
		name    string
		actorID string
		wantErr error
	}{
		{name: "inviter removes shared device", actorID: "inviter"},
		{name: "device owner withdraws sharing", actorID: "owner"},
		{name: "unrelated user is rejected", actorID: "other", wantErr: ErrUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networks := &inviteRevocationTestNetworks{invite: model.DeviceInvite{
				InviteID:      "invite-1",
				InviteCode:    "ABCD",
				InviterUserID: "inviter",
				UserID:        "owner",
				DeviceID:      "device-1",
				Status:        "accepted",
			}}
			relations := &inviteRevocationTestRelations{networks: networks}
			service := NetworkInviteService{
				Users: inviteRevocationTestUsers{users: map[string]model.User{
					"inviter": {UserID: "inviter", Email: "inviter@example.test"},
				}},
				Devices: deviceVisibilityTestDevices{items: map[string]model.Device{
					"device-1": {DeviceID: "device-1", OwnerID: "owner"},
				}},
				Networks:  networks,
				Relations: relations,
			}

			view, err := service.RevokeDeviceInvite(context.Background(), RevokeDeviceInviteInput{
				InviteID:    "invite-1",
				ActorUserID: tt.actorID,
			})
			if err != tt.wantErr {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if networks.invite.Status != "accepted" {
					t.Fatalf("unauthorized revoke changed status to %q", networks.invite.Status)
				}
				return
			}
			if view.Status != "revoked" || networks.invite.Status != "revoked" {
				t.Fatalf("expected revoked invite, view=%q stored=%q", view.Status, networks.invite.Status)
			}
			if relations.relation.Status != model.DeviceRelationStatusRevoked || relations.relation.UserID != "inviter" {
				t.Fatalf("expected revoked shared relation, got %#v", relations.relation)
			}
		})
	}
}
