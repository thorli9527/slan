package biz

import (
	"os"
	"sort"
	"strings"
)

func sortedValues[T any](values map[string]T, less func(a, b T) bool) []T {
	out := make([]T, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func optionalStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func maxInt64(value, fallback int64) int64 {
	if value < fallback {
		return fallback
	}
	return value
}

func sanitizeDNSLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "x"
	}
	return value
}

func globalDeviceDomain() string {
	domain := strings.Trim(strings.TrimSpace(os.Getenv("SLAN_GLOBAL_DEVICE_DOMAIN")), ".")
	if domain == "" {
		return "staticlss.com"
	}
	return domain
}
