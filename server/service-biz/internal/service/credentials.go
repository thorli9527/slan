package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const dummyOperatorPasswordHash = "$2y$10$/YPPP3FyuzQFSUDmZQMy9OiQljdd3yoZs03JuAH5MxEMPxH7QPjKu"

func normalizedEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizedSecret(value string) string {
	return strings.TrimSpace(value)
}

func hashPassword(raw string) (string, error) {
	raw = normalizedSecret(raw)
	if err := validateOperatorPassword(raw); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	return string(hash), err
}

func verifyPassword(hash, raw string) bool {
	hash = normalizedSecret(hash)
	raw = normalizedSecret(raw)
	if !strings.HasPrefix(hash, "$2") {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}

func validateOperatorPassword(password string) error {
	password = normalizedSecret(password)
	if len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("operator password must contain 12 to 72 characters")
	}
	normalized := strings.ToLower(password)
	for _, weak := range []string{"admin", "password", "change-me", "123456"} {
		if strings.Contains(normalized, weak) {
			return fmt.Errorf("operator password is too weak")
		}
	}
	return nil
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomBase64URL(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
