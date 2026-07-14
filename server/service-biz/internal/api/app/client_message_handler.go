package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type ClientMessageHandler struct {
	Messages       servicepkg.ClientMessageUseCase
	DeviceSessions servicepkg.DeviceSessionUseCase
}

func (h ClientMessageHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/client/messages", h.SendClientMessage),
	}
}

func (h ClientMessageHandler) SendClientMessage(w http.ResponseWriter, r *http.Request) {
	var req sendClientMessageRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, requestDeviceID(r), input.FromDeviceID)
	if !ok {
		return
	}
	input.FromDeviceID = deviceID
	item, err := h.Messages.SendClientMessage(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, item)
}
