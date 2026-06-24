package service

import (
	"testing"

	"github.com/slan/service-biz/internal/model"
)

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
