package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

// deviceOfflineFallbackDeadline 定义设备收到停用通知后未回传下线确认的最大等待时长，
// 超过后服务端兜底强制吊销授权 key。
const deviceOfflineFallbackDeadline = 24 * time.Hour

type DeviceOfflineUseCase interface {
	ReportDeviceOffline(ctx context.Context, deviceID, credentialID string) error
}

type DeviceOfflineRetryResult struct {
	Scanned     int
	Republished int
	Revoked     int
}

// DeviceOfflineService 负责设备停用后的下线确认闭环：
// 客户端确认下线（offline-report）后吊销授权 key；未确认时周期性重推 MQTT 通知并超时兜底吊销。
type DeviceOfflineService struct {
	Devices     repository.DeviceRepository
	Credentials repository.DeviceCredentialRepository
	Audit       repository.AuditRepository
	Publisher   DeviceControlPublisher
	Now         func() time.Time
}

// ReportDeviceOffline 处理客户端下线确认：标记 ack、吊销该设备的授权 key 并删除设备会话。幂等。
func (s DeviceOfflineService) ReportDeviceOffline(ctx context.Context, deviceID, credentialID string) error {
	deviceID, credentialID = strings.TrimSpace(deviceID), strings.TrimSpace(credentialID)
	if deviceID == "" {
		return ErrInvalidArgument
	}
	now := currentTime(s.Now).Unix()
	if credentialID != "" {
		item, ok, err := s.Credentials.GetDeviceCredential(ctx, credentialID)
		if err != nil {
			return err
		}
		if ok && item.DeviceID != "" && item.DeviceID != deviceID {
			return ErrInvalidArgument
		}
		if ok {
			if err := s.revokeCredentialOffline(ctx, item, now); err != nil {
				return err
			}
		}
	} else {
		items, err := s.Credentials.ListDeviceCredentials(ctx, deviceID)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.Status != model.DeviceCredentialStatusActive {
				continue
			}
			if err := s.revokeCredentialOffline(ctx, item, now); err != nil {
				return err
			}
		}
	}
	s.recordDeviceOfflineAudit(ctx, "offline_report", deviceID, now)
	return nil
}

func (s DeviceOfflineService) revokeCredentialOffline(ctx context.Context, item model.DeviceCredential, now int64) error {
	if _, err := s.Credentials.AckDeviceOffline(ctx, item.CredentialID, now); err != nil {
		return err
	}
	if item.Status == model.DeviceCredentialStatusActive {
		if _, err := s.Credentials.RevokeDeviceCredential(ctx, item.CredentialID, now); err != nil {
			return err
		}
	}
	return s.deleteDeviceSessions(ctx, item.DeviceID, item.CredentialID)
}

func (s DeviceOfflineService) deleteDeviceSessions(ctx context.Context, deviceID, credentialID string) error {
	if s.Devices == nil || strings.TrimSpace(deviceID) == "" {
		return nil
	}
	sessions, err := s.Devices.ListDeviceSessionsByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if credentialID != "" && session.CredentialID != credentialID {
			continue
		}
		if err := s.Devices.DeleteDeviceSessionByAccessToken(ctx, session.AccessToken); err != nil {
			return err
		}
	}
	return nil
}

// RetryPendingDeviceOffline 扫描待下线确认的授权 key：
// 未超时的重推 device_disabled 通知；超过 deviceOfflineFallbackDeadline 的强制吊销。
func (s DeviceOfflineService) RetryPendingDeviceOffline(ctx context.Context) (DeviceOfflineRetryResult, error) {
	var result DeviceOfflineRetryResult
	if s.Credentials == nil {
		return result, nil
	}
	pending, err := s.Credentials.ListPendingOfflineAckCredentials(ctx)
	if err != nil {
		return result, err
	}
	now := currentTime(s.Now)
	notified := map[string]bool{}
	for _, credential := range pending {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.Scanned++
		device, ok, err := s.Devices.GetDevice(ctx, credential.DeviceID)
		if err != nil {
			return result, err
		}
		if !ok || device.Status == "active" {
			continue
		}
		if now.Sub(time.Unix(credential.DisableNotifiedAt, 0)) > deviceOfflineFallbackDeadline {
			if err := s.forceRevokeOfflineCredential(ctx, credential, now.Unix()); err != nil {
				return result, err
			}
			result.Revoked++
			continue
		}
		if notified[credential.DeviceID] {
			continue
		}
		if s.Publisher != nil {
			if err := s.Publisher.PublishDeviceControl(ctx, credential.DeviceID, deviceDisabledEnvelope(credential.DeviceID, now.Unix())); err != nil {
				log.Printf("retry device disabled notification failed deviceId=%s: %v", credential.DeviceID, err)
				continue
			}
		}
		notified[credential.DeviceID] = true
		if _, err := s.Credentials.RecordDisableNotify(ctx, credential.CredentialID, now.Unix()); err != nil {
			return result, err
		}
		result.Republished++
	}
	return result, nil
}

func (s DeviceOfflineService) forceRevokeOfflineCredential(ctx context.Context, credential model.DeviceCredential, now int64) error {
	if _, err := s.Credentials.RevokeDeviceCredential(ctx, credential.CredentialID, now); err != nil {
		return err
	}
	if err := s.deleteDeviceSessions(ctx, credential.DeviceID, credential.CredentialID); err != nil {
		return err
	}
	s.recordDeviceOfflineAudit(ctx, "offline_fallback_revoke", credential.DeviceID, now)
	return nil
}

func deviceDisabledEnvelope(deviceID string, now int64) DeviceControlEnvelope {
	return DeviceControlEnvelope{
		Type:      "device_disabled",
		MessageID: fmt.Sprintf("devicedisabled%d%s", now, deviceID),
		Payload: map[string]any{
			"deviceId": deviceID,
			"reason":   "operator_disabled",
		},
	}
}

func (s DeviceOfflineService) recordDeviceOfflineAudit(ctx context.Context, action, deviceID string, now int64) {
	if s.Audit == nil {
		return
	}
	eventID, err := randomHex(16)
	if err != nil {
		return
	}
	actorType, actorID := "device", strings.TrimSpace(deviceID)
	if operatorID := AuthenticatedOperatorID(ctx); operatorID != "" {
		actorType, actorID = "operator", operatorID
	}
	_ = s.Audit.SaveAuditEvent(ctx, model.AuditEvent{
		EventID: "audit_" + eventID, ActorType: actorType, ActorID: actorID,
		Action: action, ResourceType: "device", ResourceID: strings.TrimSpace(deviceID),
		Status: "success", CreatedAt: now,
	})
}
