package biz

import (
	"crypto/sha256"
	"encoding/hex"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

func hashPassword(password string) string {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err == nil {
		return string(hashed)
	}
	return legacyPasswordHash(password)
}

func verifyPasswordHash(storedHash, password string) bool {
	storedHash = strings.TrimSpace(storedHash)
	if strings.HasPrefix(storedHash, "$2a$") || strings.HasPrefix(storedHash, "$2b$") || strings.HasPrefix(storedHash, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
	}
	return storedHash == legacyPasswordHash(password)
}

func legacyPasswordHash(password string) string {
	sum := sha256.Sum256([]byte("slan:" + password))
	return hex.EncodeToString(sum[:])
}

func bootstrapKeyHash(key string) string {
	sum := sha256.Sum256([]byte("slan:device-bootstrap:" + strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}
