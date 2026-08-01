package api

import (
	authpayload "github.com/slan/service-biz/internal/api/authpayload"
	servicepkg "github.com/slan/service-biz/internal/service"
)

func AuthSessionPayload(view servicepkg.AuthSessionView) map[string]any {
	return authpayload.LoginSession(view)
}

func AppAuthSessionPayload(view servicepkg.AuthSessionView) map[string]any {
	return AuthSessionPayload(view)
}
