package repository

import (
	"context"
	"fmt"
)

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
	if err := s.db.Delete(&gormUserSessionRecord{}, "user_id = ?", "user-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormConsoleLoginKeyRecord{}, "user_id = ?", "user-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormUserAliasRecord{}, "user_id = ?", "user-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceSessionRecord{}, "device_id = ?", "dev-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceLoginRecord{}, "device_id = ?", "dev-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceGroupAssignmentRecord{}, "device_id = ?", "dev-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormNetworkDeviceRecord{}, "network_id = ? OR device_id = ?", "net-000001", "dev-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormBootstrapKeyRecord{}, "user_id = ? OR network_id = ?", "user-000001", "net-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceInviteRecord{}, "user_id = ? OR inviter_user_id = ? OR network_id = ? OR device_id = ?", "user-000001", "user-000001", "net-000001", "dev-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormSecurityRuleRecord{}, "security_group_id IN (SELECT security_group_id FROM gorm_security_group_records WHERE network_id = ?)", "net-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormSecurityGroupRecord{}, "network_id = ?", "net-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormPublicMappingRecord{}, "network_id = ? OR device_id = ?", "net-000001", "dev-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDNSRecordRecord{}, "network_id = ?", "net-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDNSZoneRecord{}, "network_id = ?", "net-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormNetworkRecord{}, "network_id = ? AND owner_id = ?", "net-000001", "user-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormDeviceRecord{}, "device_id = ? AND owner_id = ?", "dev-000001", "user-000001").Error; err != nil {
		return err
	}
	if err := s.db.Delete(&gormUserRecord{}, "user_id = ? AND email = ?", "user-000001", "demo@example.com").Error; err != nil {
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

