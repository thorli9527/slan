package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
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
	if input.NetworkID != "" {
		return DeviceInviteView{}, ErrInvalidArgument
	}
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
	if invite.NetworkID != "" {
		return DeviceInviteView{}, ErrInvalidArgument
	}
	deviceID, err := resolveAcceptedDeviceInviteDeviceID(invite, input)
	if err != nil {
		return DeviceInviteView{}, err
	}
	if input.ActorUserID == "" {
		return DeviceInviteView{}, ErrInvalidArgument
	}
	device, err := requireManagedDevice(ctx, s.Devices, deviceID)
	if err != nil {
		return DeviceInviteView{}, err
	}
	if device.OwnerID != input.ActorUserID {
		return DeviceInviteView{}, ErrUnauthorized
	}
	if invite.InviterUserID == device.OwnerID {
		return DeviceInviteView{}, ErrConflict
	}
	if err := ensureNetworkDeviceAlias(ctx, s.Devices, s.Now, input.ActorUserID, deviceID, input.Alias); err != nil {
		return DeviceInviteView{}, err
	}
	acceptedAt := networkNow(s.Now).Unix()
	invite = acceptStandaloneInvite(invite, input.UserID, deviceID, acceptedAt)
	if s.Relations == nil {
		return DeviceInviteView{}, ErrInvalidArgument
	}
	if err := s.Relations.SaveDeviceInviteWithRelation(ctx, invite, newSharedDeviceRelation(invite, acceptedAt)); err != nil {
		return DeviceInviteView{}, err
	}
	view, err := buildDeviceInviteView(ctx, s.Users, s.Devices, invite)
	if err != nil {
		return DeviceInviteView{}, err
	}
	return view, nil
}

func (s NetworkInviteService) RevokeDeviceInvite(ctx context.Context, input RevokeDeviceInviteInput) (DeviceInviteView, error) {
	input = normalizeRevokeDeviceInviteInput(input)
	if input.InviteID == "" || input.ActorUserID == "" {
		return DeviceInviteView{}, ErrInvalidArgument
	}
	invite, ok, err := s.Networks.GetDeviceInvite(ctx, input.InviteID)
	if err != nil {
		return DeviceInviteView{}, err
	}
	if !ok {
		return DeviceInviteView{}, ErrNotFound
	}
	if invite.Status != "pending" && invite.Status != "accepted" && invite.Status != "revoked" {
		return DeviceInviteView{}, ErrConflict
	}
	allowed, err := deviceInviteCanBeRevokedBy(ctx, s.Devices, invite, input.ActorUserID)
	if err != nil {
		return DeviceInviteView{}, err
	}
	if !allowed {
		return DeviceInviteView{}, ErrUnauthorized
	}
	if invite.Status != "revoked" {
		invite = revokeDeviceInvite(invite)
		if invite.DeviceID != "" && invite.InviterUserID != "" {
			if s.Relations == nil {
				return DeviceInviteView{}, ErrInvalidArgument
			}
			if err := s.Relations.RevokeDeviceInviteWithRelation(ctx, invite, invite.InviterUserID, input.ActorUserID, networkNow(s.Now).Unix()); err != nil {
				return DeviceInviteView{}, err
			}
		} else if err := s.Networks.SaveDeviceInvite(ctx, invite); err != nil {
			return DeviceInviteView{}, err
		}
	}
	return buildDeviceInviteView(ctx, s.Users, s.Devices, invite)
}

func deviceInviteCanBeRevokedBy(
	ctx context.Context,
	devices repository.DeviceRepository,
	invite model.DeviceInvite,
	actorUserID string,
) (bool, error) {
	if actorUserID == invite.InviterUserID {
		return true, nil
	}
	if invite.Status == "pending" || invite.DeviceID == "" {
		return false, nil
	}
	device, ok, err := devices.GetDevice(ctx, invite.DeviceID)
	if err != nil {
		return false, err
	}
	return ok && device.OwnerID == actorUserID, nil
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
