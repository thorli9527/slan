package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	authrequest "github.com/slan/service-biz/internal/api/authrequest"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type AuthDeviceLoginHandler struct {
	DeviceLoginPrepare  servicepkg.AuthDeviceLoginPrepareUseCase
	DeviceLoginComplete servicepkg.AuthDeviceLoginCompleteUseCase
}

func (h AuthDeviceLoginHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/auth/device-login-devices", h.PrepareDeviceLoginDevice),
		serviceapi.NewRoute(http.MethodPost, "/api/app/auth/device-login-devices/{deviceId}/complete", h.CompleteDeviceLoginDevice),
	}
}

func (h AuthDeviceLoginHandler) PrepareDeviceLoginDevice(w http.ResponseWriter, r *http.Request) {
	var req authrequest.PrepareDeviceLoginDevice
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.ToInput()
	serviceapi.SetIfEmpty(&input.UserID, requestUserID(r))
	item, err := h.DeviceLoginPrepare.PrepareDeviceLoginDevice(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, preparedDeviceLoginPayload(item))
}

func (h AuthDeviceLoginHandler) CompleteDeviceLoginDevice(w http.ResponseWriter, r *http.Request) {
	var req authrequest.CompleteDeviceLoginDevice
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.ToInput()
	serviceapi.SetIfEmpty(&input.DeviceID, requestDeviceID(r))
	serviceapi.SetIfEmpty(&input.UserID, requestUserID(r))
	item, err := h.DeviceLoginComplete.CompleteDeviceLoginDevice(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, completedDeviceLoginPayload(item))
}
