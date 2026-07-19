package service

import "testing"

func TestManagedDeviceVirtualIP(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "managed", value: "10.23.45.67", want: true},
		{name: "managed boundary", value: "10.255.255.255", want: true},
		{name: "legacy cgnat", value: "100.124.242.246", want: false},
		{name: "empty", value: "", want: false},
		{name: "invalid", value: "pending", want: false},
		{name: "ipv6", value: "fd00::1", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := managedDeviceVirtualIP(test.value); got != test.want {
				t.Fatalf("managedDeviceVirtualIP(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}
