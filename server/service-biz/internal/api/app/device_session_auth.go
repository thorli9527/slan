package app

import (
	"net/http"
	"strings"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

func authenticatedDeviceID(
	w http.ResponseWriter,
	r *http.Request,
	sessions servicepkg.DeviceSessionUseCase,
	claimedDeviceIDs ...string,
) (string, bool) {
	session, ok := authenticatedDeviceSession(w, r, sessions, claimedDeviceIDs...)
	return session.DeviceID, ok
}

func authenticatedDeviceSession(
	w http.ResponseWriter,
	r *http.Request,
	sessions servicepkg.DeviceSessionUseCase,
	claimedDeviceIDs ...string,
) (servicepkg.DeviceSessionView, bool) {
	session, err := sessions.AuthenticateDeviceSession(r.Context(), serviceapi.AccessTokenFromRequest(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return servicepkg.DeviceSessionView{}, false
	}
	deviceID := strings.TrimSpace(session.DeviceID)
	for _, claimedDeviceID := range claimedDeviceIDs {
		claimedDeviceID = strings.TrimSpace(claimedDeviceID)
		if claimedDeviceID != "" && claimedDeviceID != deviceID {
			serviceapi.WriteError(w, servicepkg.ErrUnauthorized)
			return servicepkg.DeviceSessionView{}, false
		}
	}
	return session, true
}
