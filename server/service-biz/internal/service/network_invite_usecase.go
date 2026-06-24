package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func (s NetworkInviteService) ListDeviceInvites(ctx context.Context, userID, networkID string) ([]DeviceInviteView, error) {
	userID, networkID = normalizeDeviceInviteScope(userID, networkID)
	var (
		items []model.DeviceInvite
		err   error
	)
	switch {
	case userID != "":
		items, err = s.Networks.ListDeviceInvitesByUser(ctx, userID)
	case networkID != "":
		items, err = s.Networks.ListDeviceInvitesByNetwork(ctx, networkID)
	default:
		return []DeviceInviteView{}, nil
	}
	if err != nil {
		return nil, err
	}
	return buildDeviceInviteViews(ctx, s.Users, s.Devices, items)
}

func (s NetworkInviteService) CreateDeviceInvite(ctx context.Context, input CreateDeviceInviteInput) (DeviceInviteView, error) {
	input = normalizeCreateDeviceInviteInput(input)
	if input.UserID == "" {
		input.UserID = input.OwnerUserID
	}
	if input.UserID == "" {
		input.UserID = input.InviterUserID
	}
	if input.UserID == "" {
		return DeviceInviteView{}, ErrInvalidArgument
	}
	actorUserID := firstNonEmpty(input.InviterUserID, input.OwnerUserID, input.UserID)
	if input.NetworkID != "" {
		network, err := requireManagedNetwork(ctx, s.Networks, input.NetworkID)
		if err != nil {
			return DeviceInviteView{}, err
		}
		if actorUserID != "" {
			if _, err := requireNetworkUser(ctx, s.Users, actorUserID); err != nil {
				return DeviceInviteView{}, err
			}
			if network.OwnerID != actorUserID {
				return DeviceInviteView{}, ErrUnauthorized
			}
		}
	}
	if input.DeviceID != "" {
		device, err := requireManagedDevice(ctx, s.Devices, input.DeviceID)
		if err != nil {
			return DeviceInviteView{}, err
		}
		if actorUserID != "" && device.OwnerID != actorUserID {
			return DeviceInviteView{}, ErrUnauthorized
		}
	}
	if _, err := requireNetworkUser(ctx, s.Users, input.UserID); err != nil {
		return DeviceInviteView{}, err
	}
	ttl := 7 * 24 * time.Hour
	if input.TTLSeconds > 0 {
		ttl = time.Duration(input.TTLSeconds) * time.Second
	}
	code, err := randomHex(4)
	if err != nil {
		return DeviceInviteView{}, err
	}
	createdAt := networkNow(s.Now).Unix()
	item := newDeviceInvite(
		newManagedInviteID(s.NewInviteID),
		code,
		createdAt,
		networkNow(s.Now).Add(ttl).Unix(),
		input,
	)
	if err := s.Networks.SaveDeviceInvite(ctx, item); err != nil {
		return DeviceInviteView{}, err
	}
	return buildDeviceInviteView(ctx, s.Users, s.Devices, item)
}

func (s NetworkInviteService) AcceptDeviceInvite(ctx context.Context, input AcceptDeviceInviteInput) (DeviceInviteView, error) {
	input = normalizeAcceptDeviceInviteInput(input)
	if input.UserID == "" {
		input.UserID = input.ActorUserID
	}
	if input.InviteID == "" && input.InviteCode == "" {
		return DeviceInviteView{}, ErrInvalidArgument
	}
	var (
		invite model.DeviceInvite
		ok     bool
		err    error
	)
	if input.InviteID != "" {
		invite, ok, err = s.Networks.GetDeviceInvite(ctx, input.InviteID)
	} else {
		invite, ok, err = s.Networks.GetDeviceInviteByCode(ctx, input.InviteCode)
	}
	if err != nil {
		return DeviceInviteView{}, err
	}
	if !ok {
		return DeviceInviteView{}, ErrNotFound
	}
	if invite.Status != "pending" || invite.ExpiresAt < networkNow(s.Now).Unix() {
		return DeviceInviteView{}, ErrConflict
	}
	deviceID, err := resolveAcceptedDeviceInviteDeviceID(invite, input)
	if err != nil {
		return DeviceInviteView{}, err
	}
	if err := ensureNetworkDeviceAlias(ctx, s.Devices, s.Now, input.ActorUserID, deviceID, input.Alias); err != nil {
		return DeviceInviteView{}, err
	}
	acceptedAt := networkNow(s.Now).Unix()
	if invite.NetworkID == "" {
		invite = acceptStandaloneInvite(invite, input.UserID, deviceID, acceptedAt)
		if err := s.Networks.SaveDeviceInvite(ctx, invite); err != nil {
			return DeviceInviteView{}, err
		}
		view, err := buildDeviceInviteView(ctx, s.Users, s.Devices, invite)
		if err != nil {
			return DeviceInviteView{}, err
		}
		return view, nil
	}
	item := newNetworkDeviceMembership(invite.NetworkID, deviceID, true, acceptedAt)
	if err := s.Networks.SaveNetworkDevice(ctx, item); err != nil {
		return DeviceInviteView{}, err
	}
	invite = acceptNetworkInvite(invite, input.UserID, item, acceptedAt)
	if err := s.Networks.SaveDeviceInvite(ctx, invite); err != nil {
		return DeviceInviteView{}, err
	}
	view, err := buildDeviceInviteView(ctx, s.Users, s.Devices, invite)
	if err != nil {
		return DeviceInviteView{}, err
	}
	view.AcceptedDeviceID = item.DeviceID
	view.AcceptedUserID = invite.UserID
	return view, nil
}

func resolveAcceptedDeviceInviteDeviceID(invite model.DeviceInvite, input AcceptDeviceInviteInput) (string, error) {
	if invite.DeviceID != "" && input.DeviceID != "" && input.DeviceID != invite.DeviceID {
		return "", ErrConflict
	}
	deviceID := firstNonEmpty(input.DeviceID, invite.DeviceID)
	if deviceID == "" {
		return "", ErrInvalidArgument
	}
	return deviceID, nil
}
