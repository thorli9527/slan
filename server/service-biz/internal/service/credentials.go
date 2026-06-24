package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func normalizedEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizedSecret(value string) string {
	return strings.TrimSpace(value)
}

func hashPassword(raw string) string {
	raw = normalizedSecret(raw)
	if strings.HasPrefix(raw, "plain:") || strings.HasPrefix(raw, "sha256:") {
		return raw
	}
	sum := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func verifyPassword(hash, raw string) bool {
	hash = normalizedSecret(hash)
	raw = normalizedSecret(raw)
	switch {
	case strings.HasPrefix(hash, "plain:"):
		return strings.TrimPrefix(hash, "plain:") == raw
	case strings.HasPrefix(hash, "sha256:"):
		return hash == hashPassword(raw)
	default:
		return hash == raw
	}
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
