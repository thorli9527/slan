package authpayload

import (
	"time"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func Session(view servicepkg.AuthSessionView) map[string]any {
	user := userPayload(view.User)
	session := userSessionPayload(view.Session)
	return map[string]any{
		"user":         user,
		"session":      session,
		"userId":       sessionUserID(view),
		"email":        sessionEmail(view),
		"accessToken":  sessionAccessToken(view),
		"refreshToken": sessionRefreshToken(view),
		"expiresIn":    sessionExpiresIn(view),
	}
}

func ConsoleLoginKey(view servicepkg.ConsoleLoginKeyView) map[string]any {
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

func SessionWithDefaultNetwork(view servicepkg.AuthSessionView, defaultNetwork *servicepkg.NetworkSummaryView) map[string]any {
	payload := map[string]any{
		"auth":         Session(view),
		"accessToken":  sessionAccessToken(view),
		"refreshToken": sessionRefreshToken(view),
		"userId":       sessionUserID(view),
		"email":        sessionEmail(view),
		"expiresIn":    sessionExpiresIn(view),
	}
	if defaultNetwork != nil {
		network := defaultNetwork.Network
		payload["defaultNetwork"] = map[string]any{
			"networkId": network.NetworkID,
			"name":      network.Name,
			"code":      defaultNetwork.Code,
			"status":    network.Status,
		}
		payload["activeNetworkId"] = network.NetworkID
	}
	return payload
}

func AppSession(view servicepkg.AuthSessionView, defaultNetwork *servicepkg.NetworkSummaryView) map[string]any {
	return SessionWithDefaultNetwork(view, defaultNetwork)
}

func sessionUserID(view servicepkg.AuthSessionView) string {
	return view.User.UserID
}

func sessionEmail(view servicepkg.AuthSessionView) string {
	return view.User.Email
}

func sessionAccessToken(view servicepkg.AuthSessionView) string {
	return view.Session.AccessToken
}

func sessionRefreshToken(view servicepkg.AuthSessionView) string {
	return view.Session.RefreshToken
}

func sessionExpiresIn(view servicepkg.AuthSessionView) int64 {
	return maxInt64(view.Session.ExpiresAt-time.Now().Unix(), 0)
}

func userPayload(view servicepkg.UserView) map[string]any {
	return map[string]any{
		"userId":    view.UserID,
		"email":     view.Email,
		"name":      view.Name,
		"status":    view.Status,
		"createdAt": view.CreatedAt,
		"updatedAt": view.UpdatedAt,
	}
}

func userSessionPayload(view servicepkg.UserSessionView) map[string]any {
	return map[string]any{
		"sessionId":    view.SessionID,
		"userId":       view.UserID,
		"token":        view.AccessToken,
		"accessToken":  view.AccessToken,
		"refreshToken": view.RefreshToken,
		"status":       view.Status,
		"createdAt":    view.CreatedAt,
		"updatedAt":    view.UpdatedAt,
		"expiresAt":    view.ExpiresAt,
	}
}

func maxInt64(value, fallback int64) int64 {
	if value < fallback {
		return fallback
	}
	return value
}
