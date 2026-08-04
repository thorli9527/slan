package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type DeviceCredentialRepository interface {
	ListDeviceCredentials(ctx context.Context, deviceID string) ([]model.DeviceCredential, error)
	GetDeviceCredential(ctx context.Context, credentialID string) (model.DeviceCredential, bool, error)
	GetDeviceCredentialByKeyID(ctx context.Context, keyID string) (model.DeviceCredential, bool, error)
	BindDeviceCredential(ctx context.Context, credentialID, deviceID string, now int64) (bool, error)
	MarkDeviceCredentialUsed(ctx context.Context, credentialID, deviceID string, now int64, remoteIP string) (bool, error)
	RevokeDeviceCredential(ctx context.Context, credentialID string, now int64) (bool, error)
	DeleteInvalidDeviceCredentialsBefore(ctx context.Context, cutoff int64) (int64, error)
	SaveDeviceCredential(ctx context.Context, item model.DeviceCredential) error
}
