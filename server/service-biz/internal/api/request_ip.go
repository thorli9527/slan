package api

import (
	"net"
	"net/http"
	"os"
	"strings"
)

// RemoteIP only trusts forwarding headers when the direct peer is explicitly trusted.
func RemoteIP(r *http.Request) string {
	peer := addressHost(r.RemoteAddr)
	trusted := trustedProxyNetworks(os.Getenv("SLAN_TRUSTED_PROXY_CIDRS"))
	if peer == "" || !ipInNetworks(peer, trusted) {
		return peer
	}

	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		for index := len(parts) - 1; index >= 0; index-- {
			candidate := addressHost(parts[index])
			if candidate == "" {
				return peer
			}
			if !ipInNetworks(candidate, trusted) {
				return candidate
			}
		}
		return addressHost(parts[0])
	}
	if realIP := addressHost(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return peer
}

func addressHost(value string) string {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	ip := net.ParseIP(strings.Trim(value, "[]"))
	if ip == nil {
		return ""
	}
	return ip.String()
}

func trustedProxyNetworks(value string) []*net.IPNet {
	var networks []*net.IPNet
	for _, raw := range strings.Split(value, ",") {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil {
			networks = append(networks, network)
		}
	}
	return networks
}

func ipInNetworks(value string, networks []*net.IPNet) bool {
	ip := net.ParseIP(value)
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
