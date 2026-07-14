package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceConfigHandler struct {
	Devices        servicepkg.DeviceCoreUseCase
	DeviceSessions servicepkg.DeviceSessionUseCase
	NetworkCore    servicepkg.NetworkCoreUseCase
}

func (h DeviceConfigHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/app/devices/{deviceId}/network-configs", h.DeviceNetworkConfigs),
		serviceapi.NewRoute(http.MethodGet, "/api/app/devices/{deviceId}/mqtt-credential", h.DeviceMQTTCredential),
		serviceapi.NewRoute(http.MethodGet, "/api/app/devices/{deviceId}/mqtt-profile", h.DeviceMQTTProfile),
	}
}

func (h DeviceConfigHandler) DeviceNetworkConfigs(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, requestDeviceID(r))
	if !ok {
		return
	}
	items, err := h.NetworkCore.ResolvedDeviceNetworkConfigs(r.Context(), deviceID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, networkResolvedConfigPayload(item))
	}
	serviceapi.WriteJSON(w, http.StatusOK, networkConfigsPayload(payloads))
}

func (h DeviceConfigHandler) DeviceMQTTCredential(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, requestDeviceID(r))
	if !ok {
		return
	}
	item, err := h.Devices.DeviceMQTTCredential(r.Context(), deviceID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteEnvelope(w, http.StatusOK, "mqtt", item)
}

func (h DeviceConfigHandler) DeviceMQTTProfile(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, requestDeviceID(r))
	if !ok {
		return
	}
	item, err := h.Devices.DeviceMQTTProfile(r.Context(), deviceID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteEnvelope(w, http.StatusOK, "mqtt", item)
}
