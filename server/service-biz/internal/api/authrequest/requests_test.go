package authrequest

import "testing"

func TestLoginUserMapsDeviceID(t *testing.T) {
	input := (LoginUser{DeviceID: "device-a"}).ToInput()
	if input.DeviceID != "device-a" {
		t.Fatalf("DeviceID = %q, want device-a", input.DeviceID)
	}
}

func TestRegisterUserMapsDeviceID(t *testing.T) {
	input := (RegisterUser{DeviceID: "device-a"}).ToInput()
	if input.DeviceID != "device-a" {
		t.Fatalf("DeviceID = %q, want device-a", input.DeviceID)
	}
}
