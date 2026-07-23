package app

import (
	"context"
	"time"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func appAuthSessionPayload(ctx context.Context, networks servicepkg.NetworkCoreUseCase, view servicepkg.AuthSessionView) map[string]any {
	return appAuthSessionPayloadWithDefaultNetwork(view, appDefaultNetworkSummary(ctx, networks, view.User.UserID))
}

func appAuthSessionPayloadWithDefaultNetwork(view servicepkg.AuthSessionView, defaultNetwork *servicepkg.NetworkSummaryView) map[string]any {
	payload := map[string]any{
		"auth":         appAuthEnvelopePayload(view),
		"accessToken":  view.Session.AccessToken,
		"refreshToken": view.Session.RefreshToken,
		"userId":       view.User.UserID,
		"email":        view.User.Email,
		"expiresIn":    appSessionExpiresIn(view.Session.ExpiresAt),
	}
	if defaultNetwork != nil {
		network := defaultNetwork.Network
		payload["defaultNetwork"] = map[string]any{
			"networkId": network.NetworkID,
			"name":      network.Name,
			"status":    network.Status,
		}
		payload["activeNetworkId"] = network.NetworkID
	}
	return payload
}

func appAuthEnvelopePayload(view servicepkg.AuthSessionView) map[string]any {
	return map[string]any{
		"user":         appAuthUserPayload(view.User),
		"session":      appAuthUserSessionPayload(view.Session),
		"userId":       view.User.UserID,
		"email":        view.User.Email,
		"accessToken":  view.Session.AccessToken,
		"refreshToken": view.Session.RefreshToken,
		"expiresIn":    appSessionExpiresIn(view.Session.ExpiresAt),
	}
}

func appConsoleLoginKeyPayload(view servicepkg.ConsoleLoginKeyView) map[string]any {
	return map[string]any{
		"loginKey":  view.Key,
		"keyId":     view.KeyID,
		"userId":    view.UserID,
		"status":    view.Status,
		"expiresAt": view.ExpiresAt,
		"createdAt": view.CreatedAt,
		"updatedAt": view.UpdatedAt,
	}
}

func appDefaultNetworkSummary(ctx context.Context, networks servicepkg.NetworkCoreUseCase, userID string) *servicepkg.NetworkSummaryView {
	if networks == nil {
		return nil
	}
	items, err := networks.ListNetworks(ctx, userID)
	if err != nil || len(items) == 0 {
		return nil
	}
	return &items[0]
}

func appAuthUserPayload(view servicepkg.UserView) map[string]any {
	return map[string]any{
		"userId":    view.UserID,
		"email":     view.Email,
		"name":      view.Name,
		"status":    view.Status,
		"createdAt": view.CreatedAt,
		"updatedAt": view.UpdatedAt,
	}
}

func appAuthUserSessionPayload(view servicepkg.UserSessionView) map[string]any {
	return map[string]any{
		"sessionId":     view.SessionID,
		"userId":        view.UserID,
		"token":         view.AccessToken,
		"accessToken":   view.AccessToken,
		"refreshToken":  view.RefreshToken,
		"status":        view.Status,
		"sessionMode":   view.SessionMode,
		"createdAt":     view.CreatedAt,
		"updatedAt":     view.UpdatedAt,
		"expiresAt":     view.ExpiresAt,
		"refreshExpiry": view.RefreshExpiry,
		"revokedAt":     view.RevokedAt,
	}
}

func appSessionExpiresIn(expiresAt int64) int64 {
	remaining := expiresAt - time.Now().Unix()
	if remaining < 0 {
		return 0
	}
	return remaining
}
