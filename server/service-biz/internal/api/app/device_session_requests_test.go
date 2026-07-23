package app

import "testing"

func TestRenewDeviceSessionRequestMapsRefreshToken(t *testing.T) {
	input := (renewDeviceSessionRequest{RefreshToken: "device-refresh-token"}).toInput()
	if input.RefreshToken != "device-refresh-token" {
		t.Fatalf("RefreshToken = %q, want %q", input.RefreshToken, "device-refresh-token")
	}
}
