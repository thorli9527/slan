package repository

import (
	"context"
	"fmt"
)

const legacyDemoEmail = "demo@example.com"

func (s *GormStore) Migrate() error {
	return s.migrate()
}

func (s *GormStore) UserCount(_ context.Context) (int64, error) {
	var count int64
	if err := s.db.Model(&gormUserRecord{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *GormStore) SaveCounter(name string, value int64) error {
	return s.db.Save(&gormCounter{Name: name, Value: value}).Error
}

func (s *GormStore) DeleteLegacyDemoSeed(_ context.Context) error {
	userIDs := s.db.Model(&gormUserRecord{}).Select("user_id").Where("email = ? OR user_id = ?", legacyDemoEmail, "user-000001")
	deviceIDs := s.db.Model(&gormDeviceUserRelationRecord{}).Select("device_id").Where("user_id IN (?)", userIDs)
	networkIDs := s.db.Model(&gormNetworkRecord{}).Select("network_id").Where("owner_id IN (?) OR network_id = ?", userIDs, "net-000001")

	if err := s.db.Delete(&gormUserSessionRecord{}, "user_id IN (?)", userIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormConsoleLoginKeyRecord{}, "user_id IN (?)", userIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormUserAliasRecord{}, "user_id IN (?)", userIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceSessionRecord{}, "device_id IN (?)", deviceIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceLoginRecord{}, "device_id IN (?)", deviceIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceGroupAssignmentRecord{}, "device_id IN (?)", deviceIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormNetworkDeviceRecord{}, "network_id IN (?) OR device_id IN (?)", networkIDs, deviceIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormBootstrapKeyRecord{}, "user_id IN (?) OR network_id IN (?)", userIDs, networkIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceInviteRecord{}, "user_id IN (?) OR inviter_user_id IN (?) OR network_id IN (?) OR device_id IN (?)", userIDs, userIDs, networkIDs, deviceIDs).Error; err != nil {
		return err
	}
	securityGroupIDs := s.db.Model(&gormSecurityGroupRecord{}).Select("security_group_id").Where("network_id IN (?)", networkIDs)
	if err := s.db.Delete(&gormSecurityRuleRecord{}, "security_group_id IN (?)", securityGroupIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormSecurityGroupRecord{}, "network_id IN (?)", networkIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormPublicMappingRecord{}, "network_id IN (?) OR device_id IN (?)", networkIDs, deviceIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDNSRecordRecord{}, "network_id IN (?)", networkIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDNSZoneRecord{}, "network_id IN (?)", networkIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormNetworkRecord{}, "network_id IN (?)", networkIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceRecord{}, "device_id IN (?)", deviceIDs).Error; err != nil {
		return err
	}
	if err := s.db.Delete(
		&gormDeviceUserRelationRecord{},
		"user_id IN (?) OR device_id NOT IN (?)",
		userIDs,
		s.db.Model(&gormDeviceRecord{}).Select("device_id"),
	).Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormUserRecord{}, "user_id IN (?)", userIDs).Error; err != nil {
		return err
	}
	return nil
}

func (s *GormStore) ResetReferenceSequences() error {
	if s.db.Dialector.Name() != "postgres" {
		return nil
	}
	statements := []string{
		"SELECT setval(pg_get_serial_sequence('gorm_user_alias_records','id'), COALESCE((SELECT MAX(id) FROM gorm_user_alias_records), 1), true)",
		"SELECT setval(pg_get_serial_sequence('gorm_network_device_records','id'), COALESCE((SELECT MAX(id) FROM gorm_network_device_records), 1), true)",
	}
	for _, statement := range statements {
		if err := s.db.Exec(statement).Error; err != nil {
			return fmt.Errorf("reset sequence: %w", err)
		}
	}
	return nil
}
