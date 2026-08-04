package service

import "github.com/slan/service-biz/internal/model"

func newOpsOperator(operatorID string, input CreateOperatorInput, now int64) (model.Operator, error) {
	role := input.Role
	if role == "" {
		role = "operator"
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		return model.Operator{}, err
	}
	return model.Operator{
		OperatorID:   operatorID,
		Email:        input.Email,
		Name:         input.Name,
		PasswordHash: passwordHash,
		Role:         role,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func applyUpdateOperatorInput(item model.Operator, input UpdateOperatorInput, now int64) model.Operator {
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.Role != "" {
		item.Role = input.Role
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	item.UpdatedAt = now
	return item
}
