package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/repository"
)

type bootstrapCleanupTestDevices struct {
	repository.DeviceRepository
	cutoff int64
}

func (s *bootstrapCleanupTestDevices) DeleteExpiredDeviceBootstrapKeys(_ context.Context, expiresBefore int64) (int64, error) {
	s.cutoff = expiresBefore
	return 3, nil
}

func TestCleanupExpiredDeviceBootstrapKeysUsesTwoDayRetention(t *testing.T) {
	now := time.Date(2026, 7, 29, 14, 0, 0, 0, time.UTC)
	devices := &bootstrapCleanupTestDevices{}
	service := DeviceBootstrapKeyService{deviceBootstrapDependencies: deviceBootstrapDependencies{
		Devices: devices,
		Now:     func() time.Time { return now },
	}}

	deleted, err := service.CleanupExpiredDeviceBootstrapKeys(context.Background())
	if err != nil {
		t.Fatalf("cleanup bootstrap keys: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}
	wantCutoff := now.Add(-48 * time.Hour).Unix()
	if devices.cutoff != wantCutoff {
		t.Fatalf("cutoff = %d, want %d", devices.cutoff, wantCutoff)
	}
}
