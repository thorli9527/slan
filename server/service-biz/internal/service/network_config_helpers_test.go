package service

import "testing"

func TestManagedDeviceVirtualIP(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "managed", value: "10.23.45.67", want: true},
		{name: "first allocated", value: "10.0.1.1", want: true},
		{name: "managed boundary", value: "10.255.254.254", want: true},
		{name: "reserved system IP", value: "10.0.0.53", want: false},
		{name: "reserved subnet boundary", value: "10.0.0.254", want: false},
		{name: "network address", value: "10.1.1.0", want: false},
		{name: "broadcast address", value: "10.1.1.255", want: false},
		{name: "reserved third octet", value: "10.1.255.1", want: false},
		{name: "second pool", value: "100.124.242.246", want: true},
		{name: "second pool reserved system IP", value: "100.0.0.53", want: false},
		{name: "second pool boundary", value: "100.255.254.254", want: true},
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

func TestAllocatedDeviceVirtualIPBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		sequenceID string
		want       string
	}{
		{name: "first", sequenceID: "vip-000001", want: "10.0.1.1"},
		{name: "last host in first subnet", sequenceID: "vip-000254", want: "10.0.1.254"},
		{name: "next subnet", sequenceID: "vip-000255", want: "10.0.2.1"},
		{name: "last subnet in first tier", sequenceID: "vip-064516", want: "10.0.254.254"},
		{name: "first subnet in next tier", sequenceID: "vip-064517", want: "10.1.0.1"},
		{name: "last in first pool", sequenceID: "vip-16580866", want: "10.255.254.254"},
		{name: "first in second pool", sequenceID: "vip-16580867", want: "100.0.1.1"},
		{name: "last", sequenceID: "vip-33161732", want: "100.255.254.254"},
		{name: "exhausted", sequenceID: "vip-33161733", want: ""},
		{name: "invalid", sequenceID: "vip-invalid", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := allocatedDeviceVirtualIP(test.sequenceID); got != test.want {
				t.Fatalf("allocatedDeviceVirtualIP(%q) = %q, want %q", test.sequenceID, got, test.want)
			}
		})
	}
}
