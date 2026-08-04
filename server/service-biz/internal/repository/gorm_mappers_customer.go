package repository

import "github.com/slan/service-biz/internal/model"

func customerRecordFromModel(item model.Customer) gormCustomerRecord {
	return gormCustomerRecord{
		CustomerID: item.CustomerID,
		Email:      normalizeEmail(item.Email),
		Name:       item.Name,
		Country:    item.Country,
		Province:   item.Province,
		City:       item.City,
		IPRegion:   item.IPRegion,
		Status:     item.Status,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

func (r gormCustomerRecord) model() model.Customer {
	return model.Customer{
		CustomerID: r.CustomerID,
		Email:      r.Email,
		Name:       r.Name,
		Country:    r.Country,
		Province:   r.Province,
		City:       r.City,
		IPRegion:   r.IPRegion,
		Status:     r.Status,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
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
