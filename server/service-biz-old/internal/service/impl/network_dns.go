package impl

import (
	"fmt"
	"net"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
)

func sanitizeValues(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func sanitizeDNSWildcards(items []string) ([]string, error) {
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		host, ip, ok := strings.Cut(value, "=")
		if !ok {
			return nil, fmt.Errorf("%w: dns wildcard must use pattern=ip", ErrInvalidArgument)
		}
		host = normalizeDNSWildcardHost(host)
		ip = strings.TrimSpace(ip)
		if !isAllowedDNSWildcardHost(host) {
			return nil, fmt.Errorf("%w: dns wildcard only supports *.xx.com or *.*.xx.com style domains", ErrInvalidArgument)
		}
		if parsed := net.ParseIP(ip); parsed == nil || parsed.To4() == nil {
			return nil, fmt.Errorf("%w: dns wildcard target must be an IPv4 address", ErrInvalidArgument)
		}
		out = append(out, host+"="+ip)
	}
	return out, nil
}

func normalizeDNSWildcardHost(host string) string {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), ".")
	if strings.HasPrefix(host, "*") && !strings.HasPrefix(host, "*.") {
		host = "*." + strings.TrimPrefix(host, "*")
	}
	return host
}

func isAllowedDNSWildcardHost(host string) bool {
	if strings.Contains(host, "..") {
		return false
	}
	labels := strings.Split(host, ".")
	wildcardCount := 0
	for wildcardCount < len(labels) && labels[wildcardCount] == "*" {
		wildcardCount++
	}
	if wildcardCount == 0 || wildcardCount == len(labels) {
		return false
	}
	for _, label := range labels[wildcardCount:] {
		if label == "*" || !isDNSLabel(label) {
			return false
		}
	}
	return len(labels)-wildcardCount >= 2
}

func isDNSLabel(label string) bool {
	if label == "" || len(label) > 63 {
		return false
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, ch := range label {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func hasCreateNetworkDHCPOptions(req dto.CreateNetworkRequest) bool {
	return strings.TrimSpace(req.AllocationStartIP) != "" ||
		strings.TrimSpace(req.AllocationEndIP) != ""
}

func normalizeCreateNetworkDefaults(req dto.CreateNetworkRequest) dto.CreateNetworkRequest {
	switch strings.TrimSpace(req.CIDR) {
	case "10.0.0.0/16", "10.0.0.0/22", "10.0.0.0/24":
		req.CIDR = ""
	}
	if strings.TrimSpace(req.AllocationStartIP) == "10.0.0.2" {
		req.AllocationStartIP = ""
	}
	if strings.TrimSpace(req.AllocationEndIP) == "10.0.0.254" {
		req.AllocationEndIP = ""
	}
	return req
}
