package repository

import "github.com/slan/service-biz/internal/model"

func deviceRecordFromModel(item model.Device) gormDeviceRecord {
	return gormDeviceRecord{
		DeviceID:      item.DeviceID,
		OwnerID:       item.OwnerID,
		Name:          item.Name,
		Platform:      item.Platform,
		Alias:         item.Alias,
		OSName:        item.OSName,
		OSVersion:     item.OSVersion,
		PublicKey:     item.PublicKey,
		DeviceVersion: item.DeviceVersion,
		CountryCode:   item.CountryCode,
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
		OwnerID:       r.OwnerID,
		Name:          r.Name,
		Platform:      r.Platform,
		Alias:         r.Alias,
		OSName:        r.OSName,
		OSVersion:     r.OSVersion,
		PublicKey:     r.PublicKey,
		DeviceVersion: r.DeviceVersion,
		CountryCode:   r.CountryCode,
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
		SessionID:    item.SessionID,
		DeviceID:     item.DeviceID,
		AccessToken:  item.AccessToken,
		RefreshToken: item.RefreshToken,
		Status:       item.Status,
		ExpiresAt:    item.ExpiresAt,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func (r gormDeviceSessionRecord) model() model.DeviceSession {
	return model.DeviceSession{
		SessionID:    r.SessionID,
		DeviceID:     r.DeviceID,
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		Status:       r.Status,
		ExpiresAt:    r.ExpiresAt,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func deviceLoginRecordFromModel(item model.DeviceLoginDevice) gormDeviceLoginRecord {
	return gormDeviceLoginRecord{
		DeviceID:      item.DeviceID,
		UserID:        item.UserID,
		Name:          item.Name,
		Platform:      item.Platform,
		Alias:         item.Alias,
		OSName:        item.OSName,
		OSVersion:     item.OSVersion,
		PublicKey:     item.PublicKey,
		DeviceVersion: item.DeviceVersion,
		CountryCode:   item.CountryCode,
		VerifyCode:    item.VerifyCode,
		Status:        item.Status,
		ExpiresAt:     item.ExpiresAt,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func (r gormDeviceLoginRecord) model() model.DeviceLoginDevice {
	return model.DeviceLoginDevice{
		DeviceID:      r.DeviceID,
		UserID:        r.UserID,
		Name:          r.Name,
		Platform:      r.Platform,
		Alias:         r.Alias,
		OSName:        r.OSName,
		OSVersion:     r.OSVersion,
		PublicKey:     r.PublicKey,
		DeviceVersion: r.DeviceVersion,
		CountryCode:   r.CountryCode,
		VerifyCode:    r.VerifyCode,
		Status:        r.Status,
		ExpiresAt:     r.ExpiresAt,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

func bootstrapKeyRecordFromModel(item model.DeviceBootstrapKey) gormBootstrapKeyRecord {
	return gormBootstrapKeyRecord{
		KeyID:          item.KeyID,
		UserID:         item.UserID,
		NetworkID:      item.NetworkID,
		Name:           item.Name,
		Token:          item.Token,
		Status:         item.Status,
		ExpiresAt:      item.ExpiresAt,
		UsedAt:         item.UsedAt,
		UsedByDeviceID: item.UsedByDeviceID,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func (r gormBootstrapKeyRecord) model() model.DeviceBootstrapKey {
	return model.DeviceBootstrapKey{
		KeyID:          r.KeyID,
		UserID:         r.UserID,
		NetworkID:      r.NetworkID,
		Name:           r.Name,
		Token:          r.Token,
		Status:         r.Status,
		ExpiresAt:      r.ExpiresAt,
		UsedAt:         r.UsedAt,
		UsedByDeviceID: r.UsedByDeviceID,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func deviceGroupRecordFromModel(item model.DeviceGroup) gormDeviceGroupRecord {
	return gormDeviceGroupRecord{
		GroupID:     item.GroupID,
		UserID:      item.UserID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func (r gormDeviceGroupRecord) model() model.DeviceGroup {
	return model.DeviceGroup{
		GroupID:     r.GroupID,
		UserID:      r.UserID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}
