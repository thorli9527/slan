package biz

import "strings"

func redactedFieldName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return ""
	}
	if isSensitiveFieldName(normalized) {
		return "[redacted]"
	}
	return strings.TrimSpace(name)
}

func isSensitiveFieldName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "password") ||
		strings.Contains(name, "token") ||
		strings.Contains(name, "secret") ||
		strings.Contains(name, "key") ||
		strings.Contains(name, "authorization")
}
