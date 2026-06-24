package mqttkit

import (
	"crypto/hmac"
	"encoding/base64"
	"strings"
	"time"
)

const ServerID = "service-biz"

type AuthResult struct {
	Principal string
	DeviceID  string
}

func ValidateCredential(cfg Config, clientID, username, givenPassword string, now time.Time) (AuthResult, bool) {
	if !cfg.Enabled {
		return AuthResult{}, false
	}
	if result, ok := validateCredentialOnce(cfg, clientID, username, givenPassword, now); ok {
		return result, true
	}
	if decoded, err := base64.StdEncoding.DecodeString(givenPassword); err == nil && string(decoded) != givenPassword {
		return validateCredentialOnce(cfg, clientID, username, string(decoded), now)
	}
	return AuthResult{}, false
}

func validateCredentialOnce(cfg Config, clientID, username, givenPassword string, now time.Time) (AuthResult, bool) {
	if deviceID, ok := validateDeviceCredential(cfg, clientID, username, givenPassword, now); ok {
		return AuthResult{Principal: "device", DeviceID: deviceID}, true
	}
	if validateServerCredential(cfg, clientID, username, givenPassword, now) {
		return AuthResult{Principal: "server"}, true
	}
	return AuthResult{}, false
}

func validateDeviceCredential(cfg Config, clientID, username, givenPassword string, now time.Time) (string, bool) {
	if !cfg.Enabled {
		return "", false
	}
	deviceID, expiresAt, ok := parseDeviceUsername(cfg, username)
	if !ok || expiresAt < now.Unix() {
		return "", false
	}
	baseClientID := deviceClientID(cfg, deviceID)
	if clientID != baseClientID && !strings.HasPrefix(clientID, baseClientID+"-") {
		return "", false
	}
	return deviceID, hmac.Equal([]byte(sign(cfg.Secret, clientID, username, deviceID)), []byte(givenPassword)) ||
		hmac.Equal([]byte(sign(cfg.Secret, baseClientID, username, deviceID)), []byte(givenPassword))
}

func validateServerCredential(cfg Config, clientID, username, givenPassword string, now time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	expiresAt, ok := parseSystemUsername(cfg, username, ServerID)
	if !ok || expiresAt < now.Unix() {
		return false
	}
	baseClientID := deviceClientID(cfg, "server")
	if clientID != baseClientID && !strings.HasPrefix(clientID, baseClientID+"-") {
		return false
	}
	return hmac.Equal([]byte(sign(cfg.Secret, clientID, username, ServerID)), []byte(givenPassword)) ||
		hmac.Equal([]byte(sign(cfg.Secret, baseClientID, username, ServerID)), []byte(givenPassword))
}
