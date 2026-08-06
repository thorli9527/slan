package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceCredentialHandler struct {
	Credentials servicepkg.DeviceCredentialUseCase
	Limiter     *deviceCredentialExchangeLimiter
}

func (h DeviceCredentialHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/device-auth/token", h.Exchange),
	}
}

func (h DeviceCredentialHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key           string `json:"key"`
		DeviceID      string `json:"deviceId"`
		Platform      string `json:"platform"`
		DeviceVersion string `json:"deviceVersion"`
	}
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	remoteIP := serviceapi.RemoteIP(r)
	limitKey := deviceCredentialLimitKey(remoteIP, req.Key)
	if h.Limiter != nil && !h.Limiter.Allow(remoteIP, limitKey, time.Now()) {
		serviceapi.WriteError(w, servicepkg.ErrRateLimited)
		return
	}
	item, err := h.Credentials.ExchangeDeviceCredential(r.Context(), servicepkg.ExchangeDeviceCredentialInput{
		Key: req.Key, DeviceID: req.DeviceID, Platform: req.Platform,
		DeviceVersion: req.DeviceVersion, RemoteIP: remoteIP,
	})
	if err != nil {
		if h.Limiter != nil && errors.Is(err, servicepkg.ErrUnauthorized) {
			h.Limiter.RecordFailure(remoteIP, limitKey, time.Now())
		}
		serviceapi.WriteError(w, err)
		return
	}
	if h.Limiter != nil {
		h.Limiter.RecordSuccess(remoteIP, limitKey)
	}
	activeNetworkIDs := []string{}
	if item.Profile.ActiveNetworkID != "" {
		activeNetworkIDs = append(activeNetworkIDs, item.Profile.ActiveNetworkID)
	}
	serviceapi.WriteJSON(w, http.StatusOK, map[string]any{
		"device": appControlDeviceProfilePayload(item.Profile),
		"deviceSession": map[string]any{
			"sessionId": item.Session.SessionID, "deviceId": item.Session.DeviceID,
			"deviceToken": item.Session.AccessToken, "deviceTokenExpiresAt": item.Session.ExpiresAt,
			"deviceRefreshToken": item.Session.RefreshToken, "activeNetworkIds": activeNetworkIDs,
		},
		"mqtt": item.MQTT,
	})
}

func deviceCredentialLimitKey(remoteIP, key string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return strings.TrimSpace(remoteIP) + ":" + hex.EncodeToString(digest[:])
}
