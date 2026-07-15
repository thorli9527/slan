package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s *GormStore) ListDeviceGroups(_ context.Context, userID string) ([]model.DeviceGroup, error) {
	return listModels(s.db.Where("user_id = ?", userID).Order("group_id asc"), func(row gormDeviceGroupRecord) model.DeviceGroup {
		return row.model()
	})
}

func (s *GormStore) GetDeviceGroup(_ context.Context, groupID string) (model.DeviceGroup, bool, error) {
	return firstModel(s.db.Where("group_id = ?", groupID), func(row gormDeviceGroupRecord) model.DeviceGroup {
		return row.model()
	})
}

func (s *GormStore) SaveDeviceGroup(_ context.Context, group model.DeviceGroup) error {
	row := deviceGroupRecordFromModel(group)
	return upsertByColumns(s.db, &row, []string{"group_id"}, []string{"user_id", "name", "description", "created_at", "updated_at"})
}

func (s *GormStore) DeleteDeviceGroup(_ context.Context, groupID string) error {
	return s.db.Delete(&gormDeviceGroupRecord{}, "group_id = ?", groupID).Error
}

func (s *GormStore) SetDeviceGroups(_ context.Context, assignment model.DeviceGroupAssignment) error {
	row := gormDeviceGroupAssignmentRecord{
		DeviceID:  assignment.DeviceID,
		UserID:    assignment.UserID,
		GroupIDs:  jsonStringSlice(assignment.GroupIDs),
		UpdatedAt: assignment.UpdatedAt,
	}
	return upsertByColumns(s.db, &row, []string{"device_id", "user_id"}, []string{"group_ids", "updated_at"})
}

func (s *GormStore) ListDeviceGroupAssignments(_ context.Context, userID string) ([]model.DeviceGroupAssignment, error) {
	return listModels(s.db.Where("user_id = ?", userID).Order("device_id asc"), func(row gormDeviceGroupAssignmentRecord) model.DeviceGroupAssignment {
		return model.DeviceGroupAssignment{
			UserID:    row.UserID,
			DeviceID:  row.DeviceID,
			GroupIDs:  []string(row.GroupIDs),
			UpdatedAt: row.UpdatedAt,
		}
	})
}
