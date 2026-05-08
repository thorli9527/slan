package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/internal/repo"
)

func (s *dbState) bindDeviceUser(ctx context.Context, deviceID, userID string, boundAt int64) error {
	deviceID = strings.TrimSpace(deviceID)
	userID = strings.TrimSpace(userID)
	if deviceID == "" || userID == "" {
		return ErrInvalidArgument
	}
	if _, err := s.pg.GetDeviceByID(ctx, deviceID); err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return s.pg.BindDeviceUser(ctx, deviceID, userID, boundAt)
}

func (s *dbState) ensureInstallRegistrationAllowed(ctx context.Context, deviceID, clientIP string, now time.Time) error {
	ipAddress := normalizeInstallRegistrationIP(clientIP)
	if ipAddress == "" {
		return fmt.Errorf("%w: client IP is required", ErrInvalidArgument)
	}
	registerDay := installRegisterDay(now)
	if _, err := s.pg.GetInstallRegistration(ctx, deviceID, ipAddress, registerDay); err == nil {
		return nil
	} else if !repo.IsNotFound(err) {
		return err
	}
	count, err := s.pg.CountInstallRegisteredDevicesByIPDay(ctx, ipAddress, registerDay)
	if err != nil {
		return err
	}
	if count >= int64(fixedDeviceLimit()) {
		return fmt.Errorf("%w: install registration limit exceeded for this IP", ErrDeviceLimitExceeded)
	}
	return nil
}

func (s *dbState) recordInstallRegistration(ctx context.Context, deviceID, clientIP string, now time.Time) error {
	record := repo.DeviceInstallRegistration{
		DeviceID:     strings.TrimSpace(deviceID),
		IPAddress:    normalizeInstallRegistrationIP(clientIP),
		RegisterDay:  installRegisterDay(now),
		RegisteredAt: now.Unix(),
	}
	if record.DeviceID == "" || record.IPAddress == "" || record.RegisterDay == "" {
		return ErrInvalidArgument
	}
	if err := s.pg.InsertInstallRegistration(ctx, record); err != nil && !repo.IsUniqueViolation(err) {
		return err
	}
	return nil
}

func normalizeInstallRegistrationIP(clientIP string) string {
	return strings.TrimSpace(clientIP)
}

func installRegisterDay(now time.Time) string {
	return now.UTC().Format("2006-01-02")
}
