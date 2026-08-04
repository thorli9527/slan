package api

import (
	"net/http/httptest"
	"testing"
)

func TestRemoteIPIgnoresForwardingHeadersFromUntrustedPeer(t *testing.T) {
	t.Setenv("SLAN_TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.20")
	if got := RemoteIP(request); got != "203.0.113.10" {
		t.Fatalf("RemoteIP = %q, want direct peer", got)
	}
}

func TestRemoteIPRemovesTrustedProxyChain(t *testing.T) {
	t.Setenv("SLAN_TRUSTED_PROXY_CIDRS", "10.0.0.0/8, 192.168.0.0/16")
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.20, 192.168.1.5")
	if got := RemoteIP(request); got != "198.51.100.20" {
		t.Fatalf("RemoteIP = %q, want original client", got)
	}
}

func TestRemoteIPRejectsMalformedForwardingChain(t *testing.T) {
	t.Setenv("SLAN_TRUSTED_PROXY_CIDRS", "10.0.0.0/8")
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.20, invalid")
	if got := RemoteIP(request); got != "10.0.0.2" {
		t.Fatalf("RemoteIP = %q, want trusted peer fallback", got)
	}
}
