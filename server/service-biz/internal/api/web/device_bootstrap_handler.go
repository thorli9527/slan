package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceBootstrapHandler struct {
	DeviceBootstrap servicepkg.DeviceBootstrapUseCase
	NetworkCore     servicepkg.NetworkCoreUseCase
}

func (h DeviceBootstrapHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/device-bootstrap-keys", h.CreateDeviceBootstrapKey),
		serviceapi.NewRoute(http.MethodGet, "/api/device-bootstrap-keys", h.ListDeviceBootstrapKeys),
		serviceapi.NewRoute(http.MethodPost, "/api/device-bootstrap-keys/{keyId}/revoke", h.RevokeDeviceBootstrapKey),
		serviceapi.NewRoute(http.MethodGet, "/api/users/{userId}/device-bootstrap-keys", h.ListDeviceBootstrapKeys),
	}
}

func (h DeviceBootstrapHandler) CreateDeviceBootstrapKey(w http.ResponseWriter, r *http.Request) {
	var req createDeviceBootstrapKeyRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setUserAndActor(r, &input.UserID, &input.ActorUserID)
	item, err := h.DeviceBootstrap.CreateDeviceBootstrapKey(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, bootstrapKeyPayload(item))
}

func (h DeviceBootstrapHandler) ListDeviceBootstrapKeys(w http.ResponseWriter, r *http.Request) {
	items, err := h.DeviceBootstrap.ListDeviceBootstrapKeys(r.Context(), requestUserID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, bootstrapKeyPayload))
}

func (h DeviceBootstrapHandler) RevokeDeviceBootstrapKey(w http.ResponseWriter, r *http.Request) {
	input := revokeDeviceBootstrapKeyRequest{
		KeyID:       requestBootstrapKeyID(r),
		ActorUserID: requestActorUserID(r),
	}.toInput()
	item, err := h.DeviceBootstrap.RevokeDeviceBootstrapKey(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, bootstrapKeyPayload(item))
}
