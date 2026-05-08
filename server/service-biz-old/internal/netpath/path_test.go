package netpath

import "testing"

func TestNormalizeRelayTransport(t *testing.T) {
	if got := NormalizeRelayTransport(" UDP "); got != "udp" {
		t.Fatalf("expected udp, got %q", got)
	}
	for _, value := range []string{"tcp", "http3", "tls", "h3", "quic", "htt3"} {
		if got := NormalizeRelayTransport(value); got != "" {
			t.Fatalf("expected unsupported transport %q to be rejected, got %q", value, got)
		}
	}
}

func TestNormalizePreferredPathTypes(t *testing.T) {
	values, ok := NormalizePreferredPathTypes([]string{PathLanUdp, PathRelayUdp, PathRelayUdp, PathDerpTcpTls443})
	if !ok {
		t.Fatal("expected preferred paths to normalize")
	}
	if len(values) != 3 || values[0] != PathLanUdp || values[1] != PathRelayUdp || values[2] != PathDerpTcpTls443 {
		t.Fatalf("unexpected normalized preferred paths: %#v", values)
	}

	if _, ok := NormalizePreferredPathTypes([]string{"quic"}); ok {
		t.Fatal("expected unsupported path type to be rejected")
	}
}

func TestRelayPathTypeForTransport(t *testing.T) {
	tests := map[string]string{
		"udp": PathRelayUdp,
	}
	for transport, want := range tests {
		if got := RelayPathTypeForTransport(transport); got != want {
			t.Fatalf("transport %q: want %q, got %q", transport, want, got)
		}
	}
}

func TestIsRelayPathType(t *testing.T) {
	for _, value := range []string{"relay", "derp", PathRelayUdp, PathDerpTcpTls443} {
		if !IsRelayPathType(value) {
			t.Fatalf("expected %q to be relay path", value)
		}
	}
	if IsRelayPathType(PathDirectUdp) {
		t.Fatal("expected direct_udp to be non-relay path")
	}
}
