package app

import (
	"encoding/json"
	"net/http"
	"strings"

	serviceapi "github.com/slan/service-biz/internal/api"
	"github.com/slan/service-biz/internal/pkg/clientlogkit"
	servicepkg "github.com/slan/service-biz/internal/service"
)

const maxClientLogUploadBytes = 1 << 20

type DeviceLogHandler struct {
	DeviceSessions servicepkg.DeviceSessionUseCase
}

type deviceLogUpload struct {
	DeviceID   string            `json:"deviceId"`
	Platform   string            `json:"platform"`
	Version    string            `json:"version,omitempty"`
	CapturedAt int64             `json:"capturedAt"`
	Files      map[string]string `json:"files"`
}

func (h DeviceLogHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/devices/{deviceId}/logs", h.Upload),
	}
}

func (h DeviceLogHandler) Upload(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, requestDeviceID(r))
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxClientLogUploadBytes)
	defer r.Body.Close()
	var input deviceLogUpload
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	if strings.TrimSpace(input.DeviceID) != deviceID || strings.TrimSpace(input.Platform) == "" || len(input.Files) == 0 {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	input.DeviceID = deviceID
	name, size, err := clientlogkit.Save(deviceID, input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, map[string]any{
		"uploadId": name,
		"size":     size,
	})
}
