package service

import "context"

func (s OpsOperatorPasswordService) SetOperatorPassword(ctx context.Context, input SetOperatorPasswordInput) (OpsOperatorView, error) {
	input = normalizeSetOperatorPasswordInput(input)
	if input.OperatorID == "" || normalizedSecret(input.Password) == "" {
		return OpsOperatorView{}, ErrInvalidArgument
	}
	item, err := requireOpsOperator(ctx, s.Operators, input.OperatorID)
	if err != nil {
		return OpsOperatorView{}, err
	}
	item.PasswordHash = hashPassword(input.Password)
	item.UpdatedAt = opsNow(s.Now).Unix()
	if err := s.Operators.SaveOperator(ctx, item); err != nil {
		return OpsOperatorView{}, err
	}
	return opsOperatorView(item), nil
}

func (s OpsOperatorPasswordService) ChangePassword(ctx context.Context, input OpsChangePasswordInput) (OpsOperatorView, error) {
	return s.SetOperatorPassword(ctx, SetOperatorPasswordInput{
		OperatorID: input.OperatorID,
		Password:   input.Password,
	})
}
