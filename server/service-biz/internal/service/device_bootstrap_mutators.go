package service

import (
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func newDeviceBootstrapKey(
	devices repository.DeviceRepository,
	now time.Time,
	input CreateDeviceBootstrapKeyInput,
) (model.DeviceBootstrapKey, error) {
	token, err := randomHex(16)
	if err != nil {
		return model.DeviceBootstrapKey{}, err
	}
	expiresAt := bootstrapKeyExpiresAt(now, input)
	if expiresAt <= now.Unix() {
		return model.DeviceBootstrapKey{}, ErrInvalidArgument
	}
	nowUnix := now.Unix()
	return model.DeviceBootstrapKey{
		KeyID:     newDeviceBootstrapKeyID(devices),
		UserID:    input.UserID,
		NetworkID: input.NetworkID,
		Name:      input.Name,
		Token:     token,
		Status:    tokenStatusActive,
		ExpiresAt: expiresAt,
		CreatedAt: nowUnix,
		UpdatedAt: nowUnix,
	}, nil
}

func bootstrapKeyExpiresAt(now time.Time, input CreateDeviceBootstrapKeyInput) int64 {
	if input.ExpiresAt > 0 {
		return input.ExpiresAt
	}
	ttl := 30 * time.Minute
	ttl = installationKeyTTL(input.TTLSeconds)
	return now.Add(ttl).Unix()
}

func revokeDeviceBootstrapKey(key model.DeviceBootstrapKey, now int64) model.DeviceBootstrapKey {
	key.Status = tokenStatusRevoked
	key.UpdatedAt = now
	key.RevokedAt = now
	return key
}

func newBootstrapNetworkDevice(networkID, deviceID string, now int64) model.NetworkDevice {
	return model.NetworkDevice{
		NetworkID:      networkID,
		DeviceID:       deviceID,
		Enabled:        true,
		MemberStatus:   model.NetworkMemberStatusActive,
		PresenceStatus: model.DevicePresenceStatusOffline,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func markBootstrapKeyUsed(key model.DeviceBootstrapKey, deviceID string, now int64) model.DeviceBootstrapKey {
	key.Status = installationKeyStatusUsed
	key.UsedAt = now
	key.UsedByDeviceID = deviceID
	key.UpdatedAt = now
	return key
}
