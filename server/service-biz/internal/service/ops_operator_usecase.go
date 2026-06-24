package service

import (
	"context"
)

func (s OpsOperatorService) ListOperators(ctx context.Context) ([]OpsOperatorView, error) {
	items, err := s.Operators.ListOperators(ctx)
	if err != nil {
		return nil, err
	}
	return opsOperatorViews(items), nil
}

func (s OpsOperatorService) CreateOperator(ctx context.Context, input CreateOperatorInput) (OpsOperatorView, error) {
	input = normalizeCreateOperatorInput(input)
	if input.Email == "" || normalizedSecret(input.Password) == "" {
		return OpsOperatorView{}, ErrInvalidArgument
	}
	if _, ok, _ := s.Operators.GetOperatorByEmail(ctx, input.Email); ok {
		return OpsOperatorView{}, ErrConflict
	}
	now := opsNow(s.Now).Unix()
	item := newOpsOperator(newOpsOperatorID(s.Operators), input, now)
	if err := s.Operators.SaveOperator(ctx, item); err != nil {
		return OpsOperatorView{}, err
	}
	return opsOperatorView(item), nil
}

func (s OpsOperatorService) UpdateOperator(ctx context.Context, input UpdateOperatorInput) (OpsOperatorView, error) {
	input = normalizeUpdateOperatorInput(input)
	if input.OperatorID == "" {
		return OpsOperatorView{}, ErrInvalidArgument
	}
	item, err := requireOpsOperator(ctx, s.Operators, input.OperatorID)
	if err != nil {
		return OpsOperatorView{}, err
	}
	item = applyUpdateOperatorInput(item, input, opsNow(s.Now).Unix())
	if err := s.Operators.SaveOperator(ctx, item); err != nil {
		return OpsOperatorView{}, err
	}
	return opsOperatorView(item), nil
}
