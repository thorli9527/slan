package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s DeviceBootstrapKeyService) CreateDeviceBootstrapKey(ctx context.Context, input CreateDeviceBootstrapKeyInput) (DeviceBootstrapKeyView, error) {
	input = normalizeCreateDeviceBootstrapKeyInput(input)
	input = defaultBootstrapKeyOwner(input)
	if input.UserID == "" || input.Name == "" {
		return DeviceBootstrapKeyView{}, ErrInvalidArgument
	}
	if err := authorizeBootstrapKeyCreation(ctx, s.Users, s.Networks, input); err != nil {
		return DeviceBootstrapKeyView{}, err
	}
	key, err := newDeviceBootstrapKey(s.Devices, deviceNow(s.Now), input)
	if err != nil {
		return DeviceBootstrapKeyView{}, err
	}
	if err := s.Devices.SaveDeviceBootstrapKey(ctx, key); err != nil {
		return DeviceBootstrapKeyView{}, err
	}
	return bootstrapKeyView(key), nil
}

func (s DeviceBootstrapKeyService) ListDeviceBootstrapKeys(ctx context.Context, userID string) ([]DeviceBootstrapKeyView, error) {
	items, err := s.Devices.ListDeviceBootstrapKeys(ctx, normalizeDeviceBootstrapUserID(userID))
	if err != nil {
		return nil, err
	}
	return bootstrapKeyViews(items), nil
}

func (s DeviceBootstrapKeyService) RevokeDeviceBootstrapKey(ctx context.Context, input RevokeDeviceBootstrapKeyInput) (DeviceBootstrapKeyView, error) {
	input = normalizeRevokeDeviceBootstrapKeyInput(input)
	if input.KeyID == "" {
		return DeviceBootstrapKeyView{}, ErrInvalidArgument
	}
	key, err := requireBootstrapKeyForRevoke(ctx, s.Users, s.Devices, input)
	if err != nil {
		return DeviceBootstrapKeyView{}, err
	}
	key = revokeDeviceBootstrapKey(key, deviceNow(s.Now).Unix())
	if err := s.Devices.SaveDeviceBootstrapKey(ctx, key); err != nil {
		return DeviceBootstrapKeyView{}, err
	}
	return bootstrapKeyView(key), nil
}

func (s DeviceBootstrapSessionService) BootstrapDeviceSession(ctx context.Context, input BootstrapDeviceSessionInput) (DeviceSessionBootstrapView, error) {
	input = normalizeBootstrapDeviceSessionInput(input)
	now := deviceNow(s.Now)
	bootstrapKey, err := resolveBootstrapSessionKey(ctx, s.Devices, now.Unix(), input.InstallationKey)
	if err != nil {
		return DeviceSessionBootstrapView{}, err
	}
	input = applyBootstrapSessionOwner(input, bootstrapKey)

	var device model.Device
	if input.DeviceID != "" {
		item, err := getManagedDevice(ctx, s.Devices, input.DeviceID)
		if err != nil {
			return DeviceSessionBootstrapView{}, err
		}
		device = item
	} else {
		if input.OwnerID == "" || input.Name == "" || input.Platform == "" {
			return DeviceSessionBootstrapView{}, ErrInvalidArgument
		}
		item, err := registerManagedDevice(ctx, s.Users, s.Devices, s.Networks, s.Now, nil, RegisterDeviceInput{
			OwnerID:       input.OwnerID,
			DeviceID:      input.DeviceID,
			Name:          input.Name,
			Platform:      input.Platform,
			Alias:         input.Alias,
			OSName:        input.OSName,
			OSVersion:     input.OSVersion,
			PublicKey:     input.PublicKey,
			DeviceVersion: input.DeviceVersion,
			CountryCode:   input.CountryCode,
		})
		if err != nil {
			return DeviceSessionBootstrapView{}, err
		}
		device = item
	}
	session, err := newManagedDeviceSession(now, s.NewSessID, device.DeviceID, input.SessionMode)
	if err != nil {
		return DeviceSessionBootstrapView{}, err
	}
	if err := s.Devices.SaveDeviceSession(ctx, session); err != nil {
		return DeviceSessionBootstrapView{}, err
	}
	if bootstrapKey != nil && bootstrapKey.NetworkID != "" {
		if err := s.Networks.SaveNetworkDevice(ctx, newBootstrapNetworkDevice(bootstrapKey.NetworkID, device.DeviceID, now.Unix())); err != nil {
			return DeviceSessionBootstrapView{}, err
		}
		if _, err := bumpNetworkConfigVersion(ctx, s.Networks, nil, s.Now, bootstrapKey.NetworkID, "bootstrap_member_attached"); err != nil {
			return DeviceSessionBootstrapView{}, err
		}
	}
	if bootstrapKey != nil {
		usedAt := now.Unix()
		updatedKey := markBootstrapKeyUsed(*bootstrapKey, device.DeviceID, usedAt)
		bootstrapKey = &updatedKey
		if err := s.Devices.SaveDeviceBootstrapKey(ctx, *bootstrapKey); err != nil {
			return DeviceSessionBootstrapView{}, err
		}
	}
	return buildBootstrapDeviceSessionView(ctx, s.Users, s.Networks, s.MQTT, now, device, session)
}
