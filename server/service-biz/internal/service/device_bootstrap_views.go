package service

import "github.com/slan/service-biz/internal/model"

func deviceSessionView(item model.DeviceSession) DeviceSessionView {
	return DeviceSessionView{
		SessionID:    item.SessionID,
		DeviceID:     item.DeviceID,
		AccessToken:  item.AccessToken,
		RefreshToken: item.RefreshToken,
		Status:       item.Status,
		SessionMode:  item.SessionMode,
		ExpiresAt:    item.ExpiresAt,
		RefreshExpiry: item.RefreshExpiry,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
		RevokedAt:    item.RevokedAt,
	}
}

func bootstrapKeyView(item model.DeviceBootstrapKey) DeviceBootstrapKeyView {
	return DeviceBootstrapKeyView{
		KeyID:          item.KeyID,
		InstallationKeyID: item.KeyID,
		UserID:         item.UserID,
		Name:           item.Name,
		DeviceAlias:    item.Name,
		Token:          item.Token,
		InstallationKey: item.Token,
		Status:         item.Status,
		ExpiresAt:      item.ExpiresAt,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		NetworkID:      item.NetworkID,
		UsedAt:         item.UsedAt,
		UsedByDeviceID: item.UsedByDeviceID,
		RevokedAt:      bootstrapKeyRevokedAt(item),
	}
}

func bootstrapKeyRevokedAt(item model.DeviceBootstrapKey) int64 {
	if item.Status != "revoked" {
		return 0
	}
	return item.UpdatedAt
}
