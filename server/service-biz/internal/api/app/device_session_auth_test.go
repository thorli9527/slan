package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

type deviceSessionAuthTestUseCase struct {
	session servicepkg.DeviceSessionView
	err     error
}

func (s deviceSessionAuthTestUseCase) RenewDeviceSession(context.Context, string, servicepkg.RenewDeviceSessionInput) (servicepkg.DeviceSessionBoundView, error) {
	return servicepkg.DeviceSessionBoundView{}, nil
}

func (s deviceSessionAuthTestUseCase) AuthenticateDeviceSession(context.Context, string) (servicepkg.DeviceSessionView, error) {
	return s.session, s.err
}

func TestAuthenticatedDeviceIDRejectsClaimForDifferentDevice(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/app/devices/device-b/network-configs", nil)
	request.Header.Set("Authorization", "Bearer current-token")
	recorder := httptest.NewRecorder()

	deviceID, ok := authenticatedDeviceID(recorder, request, deviceSessionAuthTestUseCase{
		session: servicepkg.DeviceSessionView{DeviceID: "device-a"},
	}, "device-b")

	if ok || deviceID != "" {
		t.Fatalf("expected mismatched device claim to be rejected, got ok=%v deviceID=%q", ok, deviceID)
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401, got %d", recorder.Code)
	}
}

func TestAuthenticatedDeviceIDUsesCurrentSessionDevice(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/app/devices/device-a/network-configs", nil)
	request.Header.Set("Authorization", "Bearer current-token")
	recorder := httptest.NewRecorder()

	deviceID, ok := authenticatedDeviceID(recorder, request, deviceSessionAuthTestUseCase{
		session: servicepkg.DeviceSessionView{DeviceID: "device-a"},
	}, "device-a")

	if !ok || deviceID != "device-a" {
		t.Fatalf("expected current session device, got ok=%v deviceID=%q", ok, deviceID)
	}
}
