package app

import "testing"

func TestDeviceSessionRoutesOnlyExposeTokenRenewal(t *testing.T) {
	routes := DeviceSessionHandler{}.Routes()
	if len(routes) != 1 || routes[0].Key() != "POST /api/app/device/session/renew" {
		t.Fatalf("unexpected device session routes: %+v", routes)
	}
}
