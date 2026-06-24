package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type OperatorRepository interface {
	ListOperators(ctx context.Context) ([]model.Operator, error)
	GetOperator(ctx context.Context, operatorID string) (model.Operator, bool, error)
	GetOperatorByEmail(ctx context.Context, email string) (model.Operator, bool, error)
	SaveOperator(ctx context.Context, operator model.Operator) error
}

type OperatorSessionRepository interface {
	GetOperatorSessionByAccessToken(ctx context.Context, accessToken string) (model.OperatorSession, bool, error)
	SaveOperatorSession(ctx context.Context, session model.OperatorSession) error
}
