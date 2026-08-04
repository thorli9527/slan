package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
)

func (s *GormStore) ListPublicMappings(_ context.Context, networkID string) ([]model.PublicMapping, error) {
	return listModels(s.db.Where("network_id = ?", networkID).Order("mapping_id asc"), func(row gormPublicMappingRecord) model.PublicMapping {
		return row.model()
	})
}

func (s *GormStore) GetPublicMapping(_ context.Context, mappingID string) (model.PublicMapping, bool, error) {
	return firstModel(s.db.Where("mapping_id = ?", mappingID), func(row gormPublicMappingRecord) model.PublicMapping {
		return row.model()
	})
}

func (s *GormStore) SavePublicMapping(_ context.Context, mapping model.PublicMapping) error {
	row := publicMappingRecordFromModel(mapping)
	return upsertByColumns(s.db, &row, []string{"mapping_id"}, []string{"network_id", "name", "public_domain", "source_record", "device_id", "protocol", "internal_ip", "internal_port", "external_port", "access_mode", "tls_mode", "status", "created_at", "updated_at"})
}

func (s *GormStore) DeletePublicMapping(_ context.Context, mappingID string) error {
	return s.db.Delete(&gormPublicMappingRecord{}, "mapping_id = ?", mappingID).Error
}

func (s *GormStore) ListSecurityGroups(_ context.Context, networkID string) ([]model.SecurityGroup, error) {
	return listModels(s.db.Where("network_id = ?", networkID).Order("security_group_id asc"), func(row gormSecurityGroupRecord) model.SecurityGroup {
		return row.model()
	})
}

func (s *GormStore) GetSecurityGroup(_ context.Context, securityGroupID string) (model.SecurityGroup, bool, error) {
	return firstModel(s.db.Where("security_group_id = ?", securityGroupID), func(row gormSecurityGroupRecord) model.SecurityGroup {
		return row.model()
	})
}

func (s *GormStore) SaveSecurityGroup(_ context.Context, group model.SecurityGroup) error {
	row := securityGroupRecordFromModel(group)
	return upsertByColumns(s.db, &row, []string{"security_group_id"}, []string{"network_id", "name", "description", "created_at", "updated_at"})
}

func (s *GormStore) DeleteSecurityGroup(_ context.Context, securityGroupID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&gormSecurityRuleRecord{}, "security_group_id = ?", securityGroupID).Error; err != nil {
			return err
		}
		return tx.Delete(&gormSecurityGroupRecord{}, "security_group_id = ?", securityGroupID).Error
	})
}

func (s *GormStore) ListSecurityRules(_ context.Context, securityGroupID string) ([]model.SecurityRule, error) {
	return listModels(s.db.Where("security_group_id = ?", securityGroupID).Order("rule_id asc"), func(row gormSecurityRuleRecord) model.SecurityRule {
		return row.model()
	})
}

func (s *GormStore) GetSecurityRule(_ context.Context, ruleID string) (model.SecurityRule, bool, error) {
	return firstModel(s.db.Where("rule_id = ?", ruleID), func(row gormSecurityRuleRecord) model.SecurityRule {
		return row.model()
	})
}

func (s *GormStore) SaveSecurityRule(_ context.Context, rule model.SecurityRule) error {
	row := securityRuleRecordFromModel(rule)
	return upsertByColumns(s.db, &row, []string{"rule_id"}, []string{"security_group_id", "direction", "priority", "protocol", "port_range", "cidr", "peer_type", "peer_value", "action", "description", "enabled", "created_at", "updated_at"})
}

func (s *GormStore) DeleteSecurityRule(_ context.Context, ruleID string) error {
	return s.db.Delete(&gormSecurityRuleRecord{}, "rule_id = ?", ruleID).Error
}
