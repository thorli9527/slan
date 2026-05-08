package netpath

import "strings"

const (
	PathLanUdp        = "lan_udp"
	PathIPv6Udp       = "ipv6_udp"
	PathDirectUdp     = "direct_udp"
	PathRelayUdp      = "relay_udp"
	PathDerpTcpTls443 = "derp_tcp_tls_443"

	TransportUdp = "udp"
)

var canonicalPathTypes = []string{
	PathLanUdp,
	PathIPv6Udp,
	PathDirectUdp,
	PathRelayUdp,
	PathDerpTcpTls443,
}

func CanonicalPathTypes() []string {
	return append([]string(nil), canonicalPathTypes...)
}

func BandwidthSavingPreferredPathTypes() []string {
	return CanonicalPathTypes()
}

func NormalizeRelayTransport(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TransportUdp:
		return TransportUdp
	default:
		return ""
	}
}

func RelayPathTypeForTransport(value string) string {
	switch NormalizeRelayTransport(value) {
	case TransportUdp:
		return PathRelayUdp
	default:
		return ""
	}
}

func NormalizePolicyPathType(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "", "any":
		return ""
	case "direct", "p2p", "relay":
		return value
	}
	if IsCanonicalPathType(value) {
		return value
	}
	return ""
}

func IsCanonicalPathType(value string) bool {
	switch strings.TrimSpace(value) {
	case PathLanUdp, PathIPv6Udp, PathDirectUdp, PathRelayUdp, PathDerpTcpTls443:
		return true
	default:
		return false
	}
}

func IsRelayPathType(value string) bool {
	switch strings.TrimSpace(value) {
	case "relay", "derp", PathRelayUdp, PathDerpTcpTls443:
		return true
	default:
		return false
	}
}

func NormalizePreferredPathTypes(values []string) ([]string, bool) {
	if len(values) == 0 {
		return nil, true
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		pathType := strings.TrimSpace(value)
		if !IsCanonicalPathType(pathType) {
			return nil, false
		}
		if !seen[pathType] {
			seen[pathType] = true
			out = append(out, pathType)
		}
	}
	return out, true
}
