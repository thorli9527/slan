package service

import "github.com/slan/service-biz/internal/model"

func operatorView(item model.Operator) OperatorView {
	return OperatorView{
		OperatorID: item.OperatorID,
		Email:      item.Email,
		Name:       item.Name,
		Role:       item.Role,
		Status:     item.Status,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

func operatorSessionView(item model.OperatorSession) OperatorSessionView {
	return OperatorSessionView{
		SessionID:   item.SessionID,
		OperatorID:  item.OperatorID,
		AccessToken: item.AccessToken,
		Status:      "active",
		ExpiresAt:   item.ExpiresAt,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.CreatedAt,
	}
}

func opsSessionView(operator model.Operator, session model.OperatorSession) OpsSessionView {
	return OpsSessionView{
		Operator: operatorView(operator),
		Session:  operatorSessionView(session),
	}
}

func opsOperatorView(item model.Operator) OpsOperatorView {
	return OpsOperatorView{
		Operator:    operatorView(item),
		LastLoginAt: 0,
	}
}
