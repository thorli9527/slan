package service

import (
	"strings"
	"testing"
)

func TestOperatorPasswordsUseBcryptAndRejectWeakValues(t *testing.T) {
	if err := validateOperatorPassword("admin-password-123"); err == nil {
		t.Fatal("expected common weak password to be rejected")
	}
	password := "correct-horse-battery-staple"
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword returned error: %v", err)
	}
	if !strings.HasPrefix(hash, "$2") || strings.Contains(hash, password) {
		t.Fatalf("password was not stored as bcrypt: %q", hash)
	}
	if !verifyPassword(hash, password) || verifyPassword(hash, "different-strong-secret") {
		t.Fatal("bcrypt password verification returned an unexpected result")
	}
}

func TestVerifyPasswordRejectsLegacyAndPlaintextFormats(t *testing.T) {
	password := "correct-horse-battery-staple"
	for _, stored := range []string{
		password,
		"plain:" + password,
		"sha256:c8bbcb1b405bf1f9b17a1ef62c80b8c8b79cc9b06e584dda16ef8e6263efad61",
	} {
		if verifyPassword(stored, password) {
			t.Fatalf("legacy password format was accepted: %q", stored)
		}
	}
}

func TestDummyOperatorPasswordHashIsValidBcrypt(t *testing.T) {
	if !verifyPassword(dummyOperatorPasswordHash, "dummy-password-not-valid") {
		t.Fatal("dummy operator password hash is not a valid bcrypt hash")
	}
}
