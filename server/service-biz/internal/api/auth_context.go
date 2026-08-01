package api

import (
	"context"
	"strings"
)

type authenticatedUserContextKey struct{}
type authenticatedOperatorContextKey struct{}

func WithAuthenticatedUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, authenticatedUserContextKey{}, strings.TrimSpace(userID))
}

func WithAuthenticatedOperator(ctx context.Context, operatorID string) context.Context {
	return context.WithValue(ctx, authenticatedOperatorContextKey{}, strings.TrimSpace(operatorID))
}

func AuthenticatedOperatorID(ctx context.Context) string {
	operatorID, _ := ctx.Value(authenticatedOperatorContextKey{}).(string)
	return strings.TrimSpace(operatorID)
}

func AuthenticatedUserID(ctx context.Context) string {
	userID, _ := ctx.Value(authenticatedUserContextKey{}).(string)
	return strings.TrimSpace(userID)
}
