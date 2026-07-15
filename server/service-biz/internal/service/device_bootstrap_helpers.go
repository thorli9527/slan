package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func defaultBootstrapKeyOwner(input CreateDeviceBootstrapKeyInput) CreateDeviceBootstrapKeyInput {
	if input.UserID == "" {
		input.UserID = input.ActorUserID
	}
	return input
}

func authorizeBootstrapKeyCreation(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	input CreateDeviceBootstrapKeyInput,
) error {
	if _, ok, err := users.GetUser(ctx, input.UserID); err != nil {
		return err
	} else if !ok {
		return ErrNotFound
	}
	if input.ActorUserID != "" && input.ActorUserID != input.UserID {
		return ErrForbidden
	}
	if input.NetworkID == "" {
		return nil
	}
	network, err := requireDeviceNetwork(ctx, networks, input.NetworkID)
	if err != nil {
		return err
	}
	if input.ActorUserID != "" && network.OwnerID != input.ActorUserID {
		return ErrForbidden
	}
	return nil
}

func requireBootstrapKeyForRevoke(
	ctx context.Context,
	users repository.UserRepository,
	devices repository.DeviceRepository,
	input RevokeDeviceBootstrapKeyInput,
) (model.DeviceBootstrapKey, error) {
	key, ok, err := devices.GetDeviceBootstrapKey(ctx, input.KeyID)
	if err != nil {
		return model.DeviceBootstrapKey{}, err
	}
	if !ok {
		return model.DeviceBootstrapKey{}, ErrNotFound
	}
	if input.ActorUserID == "" {
		return key, nil
	}
	if _, ok, err := users.GetUser(ctx, input.ActorUserID); err != nil {
		return model.DeviceBootstrapKey{}, err
	} else if !ok {
		return model.DeviceBootstrapKey{}, ErrNotFound
	}
	if key.UserID != input.ActorUserID {
		return model.DeviceBootstrapKey{}, ErrForbidden
	}
	return key, nil
}

func resolveBootstrapSessionKey(
	ctx context.Context,
	devices repository.DeviceRepository,
	now int64,
	sessionKey string,
) (*model.DeviceBootstrapKey, error) {
	if sessionKey == "" {
		return nil, nil
	}
	key, ok, err := devices.GetDeviceBootstrapKeyByToken(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	if !ok || key.Status != "active" || (key.ExpiresAt > 0 && key.ExpiresAt < now) {
		return nil, ErrUnauthorized
	}
	return &key, nil
}

func applyBootstrapSessionOwner(input BootstrapDeviceSessionInput, key *model.DeviceBootstrapKey) BootstrapDeviceSessionInput {
	if key != nil && input.OwnerID == "" {
		input.OwnerID = key.UserID
	}
	return input
}
