package mqtt

import servicepkg "github.com/slan/service-biz/internal/service"

type authOK struct {
	TenantID string            `json:"tenantId"`
	UserID   string            `json:"userId"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

type authResponse struct {
	OK     *authOK `json:"ok,omitempty"`
	Reject string  `json:"reject,omitempty"`
}

type mqtt5AuthResponse struct {
	Success *authOK          `json:"success,omitempty"`
	Failed  *mqtt5AuthFailed `json:"failed,omitempty"`
}

type mqtt5AuthFailed struct {
	Code string `json:"code"`
}

func authAllowedPayload(view servicepkg.MQTTAuthView) *authOK {
	return &authOK{
		TenantID: view.TenantID,
		UserID:   view.IdentityID,
		Attrs: map[string]string{
			"principal": view.Principal,
			"deviceId":  view.DeviceID,
			"userId":    view.IdentityID,
		},
	}
}

func authRejectedPayload(mqtt5 bool) any {
	if mqtt5 {
		return mqtt5AuthResponse{Failed: &mqtt5AuthFailed{Code: "NotAuthorized"}}
	}
	return authResponse{Reject: "NotAuthorized"}
}

func authAcceptedPayload(mqtt5 bool, payload *authOK) any {
	if mqtt5 {
		return mqtt5AuthResponse{Success: payload}
	}
	return authResponse{OK: payload}
}

func reportEndpointPayload(changed bool) map[string]any {
	return map[string]any{
		"ok":      true,
		"changed": changed,
	}
}

func acceptedPayload() map[string]any {
	return map[string]any{
		"ok":       true,
		"accepted": true,
	}
}
