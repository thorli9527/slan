package netpath

import "testing"

func TestNormalizeRelayTransport(t *testing.T) {
	for _, value := range []string{"udp", "tcp", "http3", "tls"} {
		if got := NormalizeRelayTransport(value); got != value {
			t.Fatalf("expected %q, got %q", value, got)
		}
	}
	for _, value := range []string{"h3", "quic", "htt3"} {
		if got := NormalizeRelayTransport(value); got != "" {
			t.Fatalf("expected unsupported transport %q to be rejected, got %q", value, got)
		}
	}
}

func TestNormalizePreferredPathTypes(t *testing.T) {
	values, ok := NormalizePreferredPathTypes([]string{PathRelayUdp, PathRelayUdp, PathRelayTcp})
	if !ok {
		t.Fatal("expected preferred paths to normalize")
	}
	if len(values) != 2 || values[0] != PathRelayUdp || values[1] != PathRelayTcp {
		t.Fatalf("unexpected normalized preferred paths: %#v", values)
	}

	if _, ok := NormalizePreferredPathTypes([]string{"quic"}); ok {
		t.Fatal("expected unsupported path type to be rejected")
	}
}

func TestRelayPathTypeForTransport(t *testing.T) {
	tests := map[string]string{
		"udp":   PathRelayUdp,
		"tcp":   PathRelayTcp,
		"http3": PathRelayHttp3,
		"tls":   PathRelayTls,
	}
	for transport, want := range tests {
		if got := RelayPathTypeForTransport(transport); got != want {
			t.Fatalf("transport %q: want %q, got %q", transport, want, got)
		}
	}
}

func TestIsRelayPathType(t *testing.T) {
	for _, value := range []string{"relay", "derp", PathRelayUdp, PathRelayTcp, PathRelayHttp3, PathRelayTls} {
		if !IsRelayPathType(value) {
			t.Fatalf("expected %q to be relay path", value)
		}
	}
	if IsRelayPathType(PathDirectUdp) {
		t.Fatal("expected direct_udp to be non-relay path")
	}
}
