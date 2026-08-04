package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func (s DeviceSessionService) AuthenticateDeviceSession(ctx context.Context, accessToken string) (DeviceSessionView, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return DeviceSessionView{}, ErrUnauthorized
	}
	session, ok, err := s.Devices.GetDeviceSessionByAccessToken(ctx, accessToken)
	if err != nil {
		return DeviceSessionView{}, err
	}
	now := deviceNow(s.Now).Unix()
	if !ok || session.Status != tokenStatusActive || session.RevokedAt > 0 || session.ExpiresAt <= now {
		return DeviceSessionView{}, ErrUnauthorized
	}
	if _, err := s.activeSessionCredential(ctx, session, now); err != nil {
		return DeviceSessionView{}, err
	}
	device, err := getManagedDevice(ctx, s.Devices, session.DeviceID)
	if err != nil {
		if err == ErrNotFound {
			return DeviceSessionView{}, ErrUnauthorized
		}
		return DeviceSessionView{}, err
	}
	if device.Status != "active" {
		return DeviceSessionView{}, ErrUnauthorized
	}
	return deviceSessionView(session), nil
}

func (s DeviceSessionService) RenewDeviceSession(ctx context.Context, accessToken string, input RenewDeviceSessionInput) (DeviceSessionBoundView, error) {
	input = normalizeRenewDeviceSessionInput(input)
	if input.RefreshToken == "" {
		return DeviceSessionBoundView{}, ErrInvalidArgument
	}
	now := deviceNow(s.Now)
	session, ok, err := s.Devices.GetDeviceSessionByRefreshToken(ctx, input.RefreshToken)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	if !ok || session.Status != tokenStatusActive || session.RevokedAt > 0 || session.RefreshExpiry <= now.Unix() {
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	digest := sha256.Sum256([]byte(input.RefreshToken))
	refreshTokenHash := hex.EncodeToString(digest[:])
	rotationRetry := session.RefreshToken != input.RefreshToken
	if rotationRetry && (session.PreviousRefreshTokenHash != refreshTokenHash || session.RefreshRotationGraceExpiry < now.Unix()) {
		if session.PreviousRefreshTokenHash == refreshTokenHash && session.RefreshRotationGraceExpiry < now.Unix() {
			deleted, deleteErr := s.Devices.DeleteDeviceSessionForRefreshReuse(ctx, session.SessionID, refreshTokenHash, now.Unix())
			if deleteErr != nil {
				return DeviceSessionBoundView{}, deleteErr
			}
			if deleted {
				s.recordRefreshTokenReuse(ctx, session, input.RemoteIP, now.Unix())
			}
		}
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	credential, err := s.activeSessionCredential(ctx, session, now.Unix())
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	if !rotationRetry && accessToken != "" && normalizeDeviceAccessToken(accessToken) != session.AccessToken {
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	if !rotationRetry {
		nextSession, err := newManagedDeviceSession(now, s.NewSessID, session.DeviceID, session.SessionMode)
		if err != nil {
			return DeviceSessionBoundView{}, err
		}
		nextSession.CredentialID = session.CredentialID
		nextSession.PreviousRefreshTokenHash = refreshTokenHash
		nextSession.RefreshRotationGraceExpiry = now.Add(deviceRefreshRotationGrace).Unix()
		rotated, err := s.Devices.RotateDeviceSession(ctx, input.RefreshToken, nextSession)
		if err != nil {
			return DeviceSessionBoundView{}, err
		}
		if !rotated {
			current, found, err := s.Devices.GetDeviceSessionByRefreshToken(ctx, input.RefreshToken)
			if err != nil {
				return DeviceSessionBoundView{}, err
			}
			if !found || current.DeviceID != session.DeviceID || current.PreviousRefreshTokenHash != refreshTokenHash ||
				current.RefreshRotationGraceExpiry < now.Unix() {
				return DeviceSessionBoundView{}, ErrUnauthorized
			}
			session = current
			goto sessionRotated
		}
		used, err := s.Credentials.MarkDeviceCredentialUsed(ctx, credential.CredentialID, session.DeviceID, now.Unix(), "")
		if err != nil {
			return DeviceSessionBoundView{}, err
		}
		if !used {
			if err := s.Devices.DeleteDeviceSessionByAccessToken(ctx, nextSession.AccessToken); err != nil {
				return DeviceSessionBoundView{}, err
			}
			return DeviceSessionBoundView{}, ErrUnauthorized
		}
		session = nextSession
	}
sessionRotated:
	device, err := getManagedDevice(ctx, s.Devices, session.DeviceID)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	nowUnix := now.Unix()
	device, updated := applyRenewDeviceSessionInput(device, input, nowUnix)
	if !managedDeviceVirtualIP(device.VirtualIP) {
		device.VirtualIP, err = allocateDeviceVirtualIP(s.Devices)
		if err != nil {
			return DeviceSessionBoundView{}, err
		}
		device.UpdatedAt = nowUnix
		updated = true
	}
	if updated {
		if err := s.Devices.SaveDevice(ctx, device); err != nil {
			return DeviceSessionBoundView{}, err
		}
	}
	return buildBoundDeviceSessionView(ctx, s.Networks, s.MQTT, now, device, session)
}

func (s DeviceSessionService) recordRefreshTokenReuse(ctx context.Context, session model.DeviceSession, remoteIP string, now int64) {
	if s.Audit == nil {
		return
	}
	eventID, err := randomHex(16)
	if err != nil {
		return
	}
	_ = s.Audit.SaveAuditEvent(ctx, model.AuditEvent{
		EventID: "audit_" + eventID, ActorType: "authorization_key", ActorID: session.CredentialID,
		Action: "refresh_token_reuse", ResourceType: "device_session", ResourceID: session.SessionID,
		Status: "warning", RemoteIP: strings.TrimSpace(remoteIP), Detail: "deviceId=" + session.DeviceID,
		CreatedAt: now,
	})
}

func (s DeviceSessionService) activeSessionCredential(ctx context.Context, session model.DeviceSession, _ int64) (model.DeviceCredential, error) {
	if s.Credentials == nil || strings.TrimSpace(session.CredentialID) == "" {
		return model.DeviceCredential{}, ErrUnauthorized
	}
	credential, found, err := s.Credentials.GetDeviceCredential(ctx, session.CredentialID)
	if err != nil {
		return model.DeviceCredential{}, err
	}
	if !found || credential.DeviceID != session.DeviceID || credential.Status != model.DeviceCredentialStatusActive ||
		credential.Scopes != deviceCredentialScope {
		return model.DeviceCredential{}, ErrUnauthorized
	}
	return credential, nil
}
