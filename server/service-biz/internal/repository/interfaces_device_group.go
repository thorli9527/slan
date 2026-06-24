package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type DeviceGroupRepository interface {
	ListDeviceGroups(ctx context.Context, userID string) ([]model.DeviceGroup, error)
	GetDeviceGroup(ctx context.Context, groupID string) (model.DeviceGroup, bool, error)
	SaveDeviceGroup(ctx context.Context, group model.DeviceGroup) error
	DeleteDeviceGroup(ctx context.Context, groupID string) error
	SetDeviceGroups(ctx context.Context, assignment model.DeviceGroupAssignment) error
	ListDeviceGroupAssignments(ctx context.Context, userID string) ([]model.DeviceGroupAssignment, error)
}
