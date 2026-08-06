package ops

import (
	"net/http"
	"strings"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceCredentialHandler struct {
	Credentials servicepkg.DeviceCredentialUseCase
}

func (h DeviceCredentialHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/device-credentials", h.List),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/device-credentials", h.Create),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/devices/{deviceId}/credentials", h.ListForDevice),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/devices/{deviceId}/credentials", h.CreateForDevice),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/device-credentials/{credentialId}/revoke", h.Revoke),
	})
}

type createDeviceCredentialRequest struct {
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Scopes   string `json:"scopes"`
}

func (h DeviceCredentialHandler) List(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, strings.TrimSpace(r.URL.Query().Get("deviceId")))
}

func (h DeviceCredentialHandler) ListForDevice(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, requestDeviceID(r))
}

func (h DeviceCredentialHandler) list(w http.ResponseWriter, r *http.Request, deviceID string) {
	items, err := h.Credentials.ListDeviceCredentials(r.Context(), deviceID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, items)
}

func (h DeviceCredentialHandler) Create(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "")
}

func (h DeviceCredentialHandler) CreateForDevice(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, requestDeviceID(r))
}

func (h DeviceCredentialHandler) create(w http.ResponseWriter, r *http.Request, deviceID string) {
	var req createDeviceCredentialRequest
	if !serviceapi.DecodeJSONIfPresentOrError(w, r, &req) {
		return
	}
	if deviceID != "" {
		req.DeviceID = deviceID
	}
	item, err := h.Credentials.CreateDeviceCredential(r.Context(), servicepkg.CreateDeviceCredentialInput{
		DeviceID: req.DeviceID, Name: req.Name, Scopes: req.Scopes,
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, item)
}

func (h DeviceCredentialHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	item, err := h.Credentials.RevokeDeviceCredential(r.Context(), r.PathValue("credentialId"))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, item)
}
