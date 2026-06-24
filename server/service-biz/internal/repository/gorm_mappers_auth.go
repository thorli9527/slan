package repository

import "github.com/slan/service-biz/internal/model"

func userRecordFromModel(item model.User) gormUserRecord {
	return gormUserRecord{
		UserID:       item.UserID,
		Email:        normalizeEmail(item.Email),
		Name:         item.Name,
		Country:      item.Country,
		Province:     item.Province,
		City:         item.City,
		IPRegion:     item.IPRegion,
		PasswordHash: item.PasswordHash,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func (r gormUserRecord) model() model.User {
	return model.User{
		UserID:       r.UserID,
		Email:        r.Email,
		Name:         r.Name,
		Country:      r.Country,
		Province:     r.Province,
		City:         r.City,
		IPRegion:     r.IPRegion,
		PasswordHash: r.PasswordHash,
		Status:       r.Status,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func userSessionRecordFromModel(item model.UserSession) gormUserSessionRecord {
	return gormUserSessionRecord{
		SessionID:     item.SessionID,
		UserID:        item.UserID,
		AccessToken:   item.AccessToken,
		RefreshToken:  item.RefreshToken,
		ExpiresAt:     item.ExpiresAt,
		RefreshExpiry: item.RefreshExpiry,
		CreatedAt:     item.CreatedAt,
	}
}

func (r gormUserSessionRecord) model() model.UserSession {
	return model.UserSession{
		SessionID:     r.SessionID,
		UserID:        r.UserID,
		AccessToken:   r.AccessToken,
		RefreshToken:  r.RefreshToken,
		ExpiresAt:     r.ExpiresAt,
		RefreshExpiry: r.RefreshExpiry,
		CreatedAt:     r.CreatedAt,
	}
}

func consoleLoginKeyRecordFromModel(item model.ConsoleLoginKey) gormConsoleLoginKeyRecord {
	return gormConsoleLoginKeyRecord{
		KeyID:     item.KeyID,
		UserID:    item.UserID,
		Key:       item.Key,
		Status:    item.Status,
		ExpiresAt: item.ExpiresAt,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func (r gormConsoleLoginKeyRecord) model() model.ConsoleLoginKey {
	return model.ConsoleLoginKey{
		KeyID:     r.KeyID,
		UserID:    r.UserID,
		Key:       r.Key,
		Status:    r.Status,
		ExpiresAt: r.ExpiresAt,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func userAliasRecordFromModel(item model.UserAlias) gormUserAliasRecord {
	return gormUserAliasRecord{
		UserID:    item.UserID,
		Email:     normalizeEmail(item.Email),
		Alias:     item.Alias,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func (r gormUserAliasRecord) model() model.UserAlias {
	return model.UserAlias{
		UserID:    r.UserID,
		Email:     r.Email,
		Alias:     r.Alias,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func operatorRecordFromModel(item model.Operator) gormOperatorRecord {
	return gormOperatorRecord{
		OperatorID:   item.OperatorID,
		Email:        normalizeEmail(item.Email),
		Name:         item.Name,
		PasswordHash: item.PasswordHash,
		Role:         item.Role,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func (r gormOperatorRecord) model() model.Operator {
	return model.Operator{
		OperatorID:   r.OperatorID,
		Email:        r.Email,
		Name:         r.Name,
		PasswordHash: r.PasswordHash,
		Role:         r.Role,
		Status:       r.Status,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func operatorSessionRecordFromModel(item model.OperatorSession) gormOperatorSessionRecord {
	return gormOperatorSessionRecord{
		SessionID:   item.SessionID,
		OperatorID:  item.OperatorID,
		AccessToken: item.AccessToken,
		ExpiresAt:   item.ExpiresAt,
		CreatedAt:   item.CreatedAt,
	}
}

func (r gormOperatorSessionRecord) model() model.OperatorSession {
	return model.OperatorSession{
		SessionID:   r.SessionID,
		OperatorID:  r.OperatorID,
		AccessToken: r.AccessToken,
		ExpiresAt:   r.ExpiresAt,
		CreatedAt:   r.CreatedAt,
	}
}
