package mqttauth

import "testing"

func TestBrokerAddressDefaultsMQTTPort(t *testing.T) {
	address, err := brokerAddress("mqtt://bifromq")
	if err != nil {
		t.Fatalf("broker address: %v", err)
	}
	if address != "bifromq:1883" {
		t.Fatalf("unexpected broker address: %s", address)
	}
}

func TestBrokerAddressKeepsExplicitPort(t *testing.T) {
	address, err := brokerAddress(" mqtt://127.0.0.1:2883 ")
	if err != nil {
		t.Fatalf("broker address: %v", err)
	}
	if address != "127.0.0.1:2883" {
		t.Fatalf("unexpected broker address: %s", address)
	}
}

func TestBrokerAddressHandlesIPv6DefaultPort(t *testing.T) {
	address, err := brokerAddress("mqtt://[::1]")
	if err != nil {
		t.Fatalf("broker address: %v", err)
	}
	if address != "[::1]:1883" {
		t.Fatalf("unexpected broker address: %s", address)
	}
}

func TestBrokerAddressRejectsMissingHost(t *testing.T) {
	if _, err := brokerAddress("mqtt://"); err == nil {
		t.Fatal("expected missing host to fail")
	}
}
