package repository

import "github.com/slan/service-biz/internal/model"

func deviceRecordFromModel(item model.Device) gormDeviceRecord {
	return gormDeviceRecord{
		DeviceID:      item.DeviceID,
		VirtualIP:     item.VirtualIP,
		Name:          item.Name,
		Platform:      item.Platform,
		Alias:         item.Alias,
		OSName:        item.OSName,
		OSVersion:     item.OSVersion,
		PublicKey:     item.PublicKey,
		DeviceVersion: item.DeviceVersion,
		PublicIP:      item.PublicIP,
		CountryCode:   item.CountryCode,
		CityCode:      item.CityCode,
		GeoUpdatedAt:  item.GeoUpdatedAt,
		RXBytesTotal:  item.RXBytesTotal,
		TXBytesTotal:  item.TXBytesTotal,
		Status:        item.Status,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		LastSeenAt:    item.LastSeenAt,
	}
}

func (r gormDeviceRecord) model() model.Device {
	return model.Device{
		DeviceID:      r.DeviceID,
		VirtualIP:     r.VirtualIP,
		Name:          r.Name,
		Platform:      r.Platform,
		Alias:         r.Alias,
		OSName:        r.OSName,
		OSVersion:     r.OSVersion,
		PublicKey:     r.PublicKey,
		DeviceVersion: r.DeviceVersion,
		PublicIP:      r.PublicIP,
		CountryCode:   r.CountryCode,
		CityCode:      r.CityCode,
		GeoUpdatedAt:  r.GeoUpdatedAt,
		RXBytesTotal:  r.RXBytesTotal,
		TXBytesTotal:  r.TXBytesTotal,
		Status:        r.Status,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
		LastSeenAt:    r.LastSeenAt,
	}
}

func deviceSessionRecordFromModel(item model.DeviceSession) gormDeviceSessionRecord {
	return gormDeviceSessionRecord{
		SessionID:                  item.SessionID,
		DeviceID:                   item.DeviceID,
		CredentialID:               item.CredentialID,
		AccessToken:                item.AccessToken,
		RefreshToken:               item.RefreshToken,
		Status:                     item.Status,
		SessionMode:                item.SessionMode,
		PreviousRefreshTokenHash:   item.PreviousRefreshTokenHash,
		RefreshRotationGraceExpiry: item.RefreshRotationGraceExpiry,
		ExpiresAt:                  item.ExpiresAt,
		RefreshExpiry:              item.RefreshExpiry,
		CreatedAt:                  item.CreatedAt,
		UpdatedAt:                  item.UpdatedAt,
		RevokedAt:                  item.RevokedAt,
	}
}

func (r gormDeviceSessionRecord) model() model.DeviceSession {
	return model.DeviceSession{
		SessionID:                  r.SessionID,
		DeviceID:                   r.DeviceID,
		CredentialID:               r.CredentialID,
		AccessToken:                r.AccessToken,
		RefreshToken:               r.RefreshToken,
		Status:                     r.Status,
		SessionMode:                r.SessionMode,
		PreviousRefreshTokenHash:   r.PreviousRefreshTokenHash,
		RefreshRotationGraceExpiry: r.RefreshRotationGraceExpiry,
		ExpiresAt:                  r.ExpiresAt,
		RefreshExpiry:              r.RefreshExpiry,
		CreatedAt:                  r.CreatedAt,
		UpdatedAt:                  r.UpdatedAt,
		RevokedAt:                  r.RevokedAt,
	}
}

func deviceGroupRecordFromModel(item model.DeviceGroup) gormDeviceGroupRecord {
	return gormDeviceGroupRecord{
		GroupID:     item.GroupID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func (r gormDeviceGroupRecord) model() model.DeviceGroup {
	return model.DeviceGroup{
		GroupID:     r.GroupID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}
