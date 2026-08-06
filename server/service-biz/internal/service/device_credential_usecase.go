package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

const (
	deviceCredentialLegacyKeyPrefix  = "slan_device_"
	deviceCredentialScope            = "standard_device"
	deviceCredentialKeyIDSize        = 8
	deviceCredentialSecretSize       = 18
	deviceCredentialBindingTTL       = 30 * time.Minute
	invalidDeviceCredentialRetention = 7 * 24 * time.Hour
	defaultDeviceCredentialName      = "设备授权 Key"
)

type DeviceCredentialService struct {
	Devices         repository.DeviceRepository
	Credentials     repository.DeviceCredentialRepository
	Audit           repository.AuditRepository
	Networks        repository.NetworkRepository
	MQTT            mqttkit.Config
	Pepper          string
	PreviousPeppers []string
	NewSessID       func(string) string
	Now             func() time.Time
}

func (s DeviceCredentialService) CreateDeviceCredential(ctx context.Context, input CreateDeviceCredentialInput) (CreatedDeviceCredentialView, error) {
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	input.Name = strings.TrimSpace(input.Name)
	input.Scopes = strings.TrimSpace(input.Scopes)
	if input.Name == "" {
		input.Name = defaultDeviceCredentialName
	}
	if strings.TrimSpace(s.Pepper) == "" {
		return CreatedDeviceCredentialView{}, ErrInvalidArgument
	}
	if input.Scopes == "" {
		input.Scopes = deviceCredentialScope
	}
	if input.Scopes != deviceCredentialScope {
		return CreatedDeviceCredentialView{}, ErrInvalidArgument
	}
	if input.DeviceID != "" {
		if _, err := requireOpsDevice(ctx, s.Devices, input.DeviceID); err != nil {
			return CreatedDeviceCredentialView{}, err
		}
		if err := s.revokeOtherActiveDeviceCredentials(ctx, input.DeviceID, ""); err != nil {
			return CreatedDeviceCredentialView{}, err
		}
	}
	now := currentTime(s.Now).Unix()
	keyID, err := randomHex(deviceCredentialKeyIDSize)
	if err != nil {
		return CreatedDeviceCredentialView{}, err
	}
	secret, err := randomBase64URL(deviceCredentialSecretSize)
	if err != nil {
		return CreatedDeviceCredentialView{}, err
	}
	item := model.DeviceCredential{
		CredentialID: "dcred_" + keyID,
		KeyID:        keyID, DeviceID: input.DeviceID, Name: input.Name,
		SecretHash: credentialSecretHash(s.Pepper, secret), Status: model.DeviceCredentialStatusActive,
		Scopes: input.Scopes, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Credentials.SaveDeviceCredential(ctx, item); err != nil {
		return CreatedDeviceCredentialView{}, err
	}
	s.recordCredentialAudit(ctx, "create", item.CredentialID, "success", "", now)
	return CreatedDeviceCredentialView{DeviceCredentialView: deviceCredentialView(item), Key: keyID + "_" + secret}, nil
}

func (s DeviceCredentialService) ListDeviceCredentials(ctx context.Context, deviceID string) ([]DeviceCredentialView, error) {
	items, err := s.Credentials.ListDeviceCredentials(ctx, strings.TrimSpace(deviceID))
	if err != nil {
		return nil, err
	}
	views := make([]DeviceCredentialView, 0, len(items))
	for _, item := range items {
		views = append(views, deviceCredentialView(item))
	}
	return views, nil
}

func (s DeviceCredentialService) CleanupInvalidDeviceCredentials(ctx context.Context) (int64, error) {
	now := currentTime(s.Now)
	expired, err := s.Credentials.DeleteExpiredUnboundDeviceCredentialsBefore(ctx, now.Add(-deviceCredentialBindingTTL).Unix())
	if err != nil {
		return 0, err
	}
	invalid, err := s.Credentials.DeleteInvalidDeviceCredentialsBefore(ctx, now.Add(-invalidDeviceCredentialRetention).Unix())
	return expired + invalid, err
}

func (s DeviceCredentialService) RevokeDeviceCredential(ctx context.Context, credentialID string) (DeviceCredentialView, error) {
	item, ok, err := s.Credentials.GetDeviceCredential(ctx, strings.TrimSpace(credentialID))
	if err != nil {
		return DeviceCredentialView{}, err
	}
	if !ok {
		return DeviceCredentialView{}, ErrNotFound
	}
	now := currentTime(s.Now).Unix()
	revoked, err := s.Credentials.RevokeDeviceCredential(ctx, item.CredentialID, now)
	if err != nil {
		return DeviceCredentialView{}, err
	}
	if !revoked {
		return DeviceCredentialView{}, ErrNotFound
	}
	item.Status, item.RevokedAt, item.UpdatedAt = model.DeviceCredentialStatusRevoked, now, now
	sessions, err := s.Devices.ListDeviceSessionsByDeviceID(ctx, item.DeviceID)
	if err != nil {
		return DeviceCredentialView{}, err
	}
	for _, session := range sessions {
		if session.CredentialID == item.CredentialID {
			if err := s.Devices.DeleteDeviceSessionByAccessToken(ctx, session.AccessToken); err != nil {
				return DeviceCredentialView{}, err
			}
		}
	}
	s.recordCredentialAudit(ctx, "revoke", item.CredentialID, "success", "", now)
	return deviceCredentialView(item), nil
}

func (s DeviceCredentialService) ExchangeDeviceCredential(ctx context.Context, input ExchangeDeviceCredentialInput) (DeviceSessionBoundView, error) {
	keyID, _, _ := parseDeviceCredentialKey(input.Key)
	view, err := s.exchangeDeviceCredential(ctx, input)
	status := "success"
	if err != nil {
		status = "failure"
	}
	s.recordCredentialAudit(ctx, "exchange", keyID, status, input.RemoteIP, currentTime(s.Now).Unix())
	return view, err
}

func (s DeviceCredentialService) exchangeDeviceCredential(ctx context.Context, input ExchangeDeviceCredentialInput) (DeviceSessionBoundView, error) {
	input.Platform = strings.ToLower(strings.TrimSpace(input.Platform))
	input.DeviceVersion = strings.TrimSpace(input.DeviceVersion)
	keyID, secret, ok := parseDeviceCredentialKey(input.Key)
	if !ok || strings.TrimSpace(s.Pepper) == "" {
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	credential, found, err := s.Credentials.GetDeviceCredentialByKeyID(ctx, keyID)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	now := currentTime(s.Now)
	if !found || credential.Status != model.DeviceCredentialStatusActive ||
		!credentialSecretMatches(credential.SecretHash, secret, s.Pepper, s.PreviousPeppers) {
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	bindingCutoff := now.Add(-deviceCredentialBindingTTL).Unix()
	if credential.DeviceID == "" && credential.CreatedAt <= bindingCutoff {
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	requested := strings.TrimSpace(input.DeviceID)
	if credential.DeviceID != "" && requested != "" && requested != credential.DeviceID {
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	if credential.DeviceID == "" {
		if requested == "" {
			return DeviceSessionBoundView{}, ErrUnauthorized
		}
		if err := s.revokeOtherActiveDeviceCredentials(ctx, requested, credential.CredentialID); err != nil {
			return DeviceSessionBoundView{}, err
		}
		bound, err := s.Credentials.BindDeviceCredential(ctx, credential.CredentialID, requested, bindingCutoff, now.Unix())
		if err != nil {
			return DeviceSessionBoundView{}, err
		}
		if !bound {
			credential, found, err = s.Credentials.GetDeviceCredential(ctx, credential.CredentialID)
			if err != nil {
				return DeviceSessionBoundView{}, err
			}
			if !found || credential.DeviceID != requested {
				return DeviceSessionBoundView{}, ErrUnauthorized
			}
		} else {
			credential.DeviceID = requested
		}
	}
	deviceID := credential.DeviceID
	if err := s.revokeOtherActiveDeviceCredentials(ctx, deviceID, credential.CredentialID); err != nil {
		return DeviceSessionBoundView{}, err
	}
	device, found, err := s.Devices.GetDevice(ctx, deviceID)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	deviceChanged := !found
	if !found {
		platform := input.Platform
		if platform == "" {
			platform = "unknown"
		}
		device = model.Device{
			DeviceID: deviceID, Name: credential.Name, Alias: credential.Name,
			Platform: platform, DeviceVersion: input.DeviceVersion,
			Status: "active", CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
		}
	} else {
		if device.Status != "active" {
			device.Status = "active"
			deviceChanged = true
		}
		if input.Platform != "" && device.Platform != input.Platform {
			device.Platform = input.Platform
			deviceChanged = true
		}
		if input.DeviceVersion != "" && device.DeviceVersion != input.DeviceVersion {
			device.DeviceVersion = input.DeviceVersion
			deviceChanged = true
		}
		if deviceChanged {
			device.UpdatedAt = now.Unix()
		}
	}
	if !managedDeviceVirtualIP(device.VirtualIP) {
		if err := s.assignAndSaveDeviceVirtualIP(ctx, &device); err != nil {
			return DeviceSessionBoundView{}, err
		}
	} else if deviceChanged {
		if err := s.Devices.SaveDevice(ctx, device); err != nil {
			return DeviceSessionBoundView{}, err
		}
	}
	session, err := newManagedDeviceSession(now, s.NewSessID, device.DeviceID, tokenModeLong)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	session.CredentialID = credential.CredentialID
	if err := replaceDeviceSession(ctx, s.Devices, session); err != nil {
		return DeviceSessionBoundView{}, err
	}
	used, err := s.Credentials.MarkDeviceCredentialUsed(ctx, credential.CredentialID, deviceID, now.Unix(), strings.TrimSpace(input.RemoteIP))
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	if !used {
		if err := s.Devices.DeleteDeviceSessionByAccessToken(ctx, session.AccessToken); err != nil {
			return DeviceSessionBoundView{}, err
		}
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	remoteIP := strings.TrimSpace(input.RemoteIP)
	if previousIP := strings.TrimSpace(credential.LastUsedIP); previousIP != "" && remoteIP != "" && previousIP != remoteIP {
		s.recordCredentialSourceChange(ctx, credential.CredentialID, previousIP, remoteIP, now.Unix())
	}
	return buildBoundDeviceSessionView(ctx, s.Networks, s.MQTT, now, device, session)
}

func (s DeviceCredentialService) revokeOtherActiveDeviceCredentials(ctx context.Context, deviceID, keepCredentialID string) error {
	items, err := s.Credentials.ListDeviceCredentials(ctx, strings.TrimSpace(deviceID))
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Status != model.DeviceCredentialStatusActive || item.CredentialID == keepCredentialID {
			continue
		}
		if _, err := s.RevokeDeviceCredential(ctx, item.CredentialID); err != nil {
			return err
		}
	}
	return nil
}

func (s DeviceCredentialService) assignAndSaveDeviceVirtualIP(ctx context.Context, device *model.Device) error {
	const maxAttempts = 32
	for attempt := 0; attempt < maxAttempts; attempt++ {
		virtualIP, err := allocateDeviceVirtualIP(s.Devices)
		if err != nil {
			return err
		}
		device.VirtualIP = virtualIP
		if err := s.Devices.SaveDevice(ctx, *device); err != nil {
			if errors.Is(err, repository.ErrDeviceVirtualIPConflict) {
				continue
			}
			return err
		}
		return nil
	}
	return conflictError("failed to allocate a unique device virtual IP")
}

func (s DeviceCredentialService) recordCredentialSourceChange(ctx context.Context, credentialID, previousIP, remoteIP string, now int64) {
	if s.Audit == nil {
		return
	}
	eventID, err := randomHex(16)
	if err != nil {
		return
	}
	_ = s.Audit.SaveAuditEvent(ctx, model.AuditEvent{
		EventID: "audit_" + eventID, ActorType: "authorization_key", ActorID: credentialID,
		Action: "exchange_source_changed", ResourceType: "device_credential", ResourceID: credentialID,
		Status: "warning", RemoteIP: remoteIP, Detail: "source changed from " + previousIP + " to " + remoteIP,
		CreatedAt: now,
	})
}

func (s DeviceCredentialService) recordCredentialAudit(ctx context.Context, action, resourceID, status, remoteIP string, now int64) {
	if s.Audit == nil {
		return
	}
	eventID, err := randomHex(16)
	if err != nil {
		return
	}
	actorType, actorID := "authorization_key", strings.TrimSpace(resourceID)
	if operatorID := AuthenticatedOperatorID(ctx); operatorID != "" {
		actorType, actorID = "operator", operatorID
	}
	_ = s.Audit.SaveAuditEvent(ctx, model.AuditEvent{
		EventID: "audit_" + eventID, ActorType: actorType, ActorID: actorID,
		Action: action, ResourceType: "device_credential", ResourceID: strings.TrimSpace(resourceID),
		Status: status, RemoteIP: strings.TrimSpace(remoteIP), CreatedAt: now,
	})
}

func parseDeviceCredentialKey(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, deviceCredentialLegacyKeyPrefix)
	parts := strings.SplitN(value, "_", 2)
	returnValue := len(parts) == 2 && parts[0] != "" && parts[1] != ""
	if !returnValue {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func credentialSecretHash(pepper, secret string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(secret))
	return hex.EncodeToString(mac.Sum(nil))
}

func credentialSecretMatches(storedHash, secret, currentPepper string, previousPeppers []string) bool {
	matched := false
	for _, pepper := range append([]string{currentPepper}, previousPeppers...) {
		pepper = strings.TrimSpace(pepper)
		if pepper == "" {
			continue
		}
		if hmac.Equal([]byte(storedHash), []byte(credentialSecretHash(pepper, secret))) {
			matched = true
		}
	}
	return matched
}

func deviceCredentialView(item model.DeviceCredential) DeviceCredentialView {
	return DeviceCredentialView{CredentialID: item.CredentialID, KeyID: item.KeyID, DeviceID: item.DeviceID, Name: item.Name,
		Scopes: item.Scopes, Status: item.Status, LastUsedAt: item.LastUsedAt,
		LastUsedIP: item.LastUsedIP, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, RevokedAt: item.RevokedAt}
}
