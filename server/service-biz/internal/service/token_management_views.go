package service

import "github.com/slan/service-biz/internal/model"

func userManagedSessionView(item model.UserSession) UserManagedSessionView {
	return UserManagedSessionView{
		SessionID:     item.SessionID,
		UserID:        item.UserID,
		Status:        item.Status,
		SessionMode:   item.SessionMode,
		ExpiresAt:     item.ExpiresAt,
		RefreshExpiry: item.RefreshExpiry,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		RevokedAt:     item.RevokedAt,
	}
}

func deviceManagedSessionView(item model.DeviceSession) DeviceManagedSessionView {
	return DeviceManagedSessionView{
		SessionID:     item.SessionID,
		DeviceID:      item.DeviceID,
		Status:        item.Status,
		SessionMode:   item.SessionMode,
		ExpiresAt:     item.ExpiresAt,
		RefreshExpiry: item.RefreshExpiry,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		RevokedAt:     item.RevokedAt,
	}
}
