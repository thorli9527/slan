package service

import "context"

type operatorContextKey struct{}
type operatorRoleContextKey struct{}

func WithAuthenticatedOperator(ctx context.Context, operatorID string) context.Context {
	return context.WithValue(ctx, operatorContextKey{}, operatorID)
}

func AuthenticatedOperatorID(ctx context.Context) string {
	operatorID, _ := ctx.Value(operatorContextKey{}).(string)
	return operatorID
}

func WithAuthenticatedOperatorRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, operatorRoleContextKey{}, role)
}

func AuthenticatedOperatorRole(ctx context.Context) string {
	role, _ := ctx.Value(operatorRoleContextKey{}).(string)
	return role
}
