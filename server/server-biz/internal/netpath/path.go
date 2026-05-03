package netpath

import "strings"

const (
	PathDirectUdp  = "direct_udp"
	PathRelayUdp   = "relay_udp"
	PathRelayTcp   = "relay_tcp"
	PathRelayHttp3 = "relay_http3"
	PathRelayTls   = "relay_tls"

	TransportUdp   = "udp"
	TransportTcp   = "tcp"
	TransportHttp3 = "http3"
	TransportTls   = "tls"
)

var canonicalPathTypes = []string{
	PathDirectUdp,
	PathRelayUdp,
	PathRelayTcp,
	PathRelayHttp3,
	PathRelayTls,
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
	case TransportTcp:
		return TransportTcp
	case TransportHttp3:
		return TransportHttp3
	case TransportTls:
		return TransportTls
	default:
		return ""
	}
}

func RelayPathTypeForTransport(value string) string {
	switch NormalizeRelayTransport(value) {
	case TransportUdp:
		return PathRelayUdp
	case TransportTcp:
		return PathRelayTcp
	case TransportHttp3:
		return PathRelayHttp3
	case TransportTls:
		return PathRelayTls
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
	case PathDirectUdp, PathRelayUdp, PathRelayTcp, PathRelayHttp3, PathRelayTls:
		return true
	default:
		return false
	}
}

func IsRelayPathType(value string) bool {
	switch strings.TrimSpace(value) {
	case "relay", "derp", PathRelayUdp, PathRelayTcp, PathRelayHttp3, PathRelayTls:
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
