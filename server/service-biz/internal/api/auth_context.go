package api

import (
	"context"
	"strings"
)

type authenticatedUserContextKey struct{}

func WithAuthenticatedUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, authenticatedUserContextKey{}, strings.TrimSpace(userID))
}

func AuthenticatedUserID(ctx context.Context) string {
	userID, _ := ctx.Value(authenticatedUserContextKey{}).(string)
	return strings.TrimSpace(userID)
}
