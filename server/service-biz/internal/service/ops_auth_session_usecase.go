package service

import "context"

func (s OpsAuthSessionService) Login(ctx context.Context, input OpsLoginInput) (OpsSessionView, error) {
	operator, ok, err := s.Operators.GetOperatorByEmail(ctx, input.Email)
	if err != nil {
		return OpsSessionView{}, err
	}
	if !ok || !verifyPassword(operator.PasswordHash, input.Password) {
		return OpsSessionView{}, ErrUnauthorized
	}
	session, err := newOperatorSession(opsNow(s.Now), s.NewOperatorSessID, operator.OperatorID)
	if err != nil {
		return OpsSessionView{}, err
	}
	if err := s.OperatorSessions.SaveOperatorSession(ctx, session); err != nil {
		return OpsSessionView{}, err
	}
	return opsSessionView(operator, session), nil
}
