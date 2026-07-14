package repository

func (s *GormStore) migrate() error {
	if err := s.db.AutoMigrate(
		&gormCounter{},
		&gormUserRecord{},
		&gormUserSessionRecord{},
		&gormConsoleLoginKeyRecord{},
		&gormUserAliasRecord{},
		&gormDeviceRecord{},
		&gormDeviceLoginRecord{},
		&gormDeviceSessionRecord{},
		&gormBootstrapKeyRecord{},
		&gormDeviceGroupRecord{},
		&gormDeviceGroupAssignmentRecord{},
		&gormNetworkRecord{},
		&gormNetworkConfigVersionRecord{},
		&gormNetworkDeviceRecord{},
		&gormNetworkDeviceGroupReferenceRecord{},
		&gormDeviceInviteRecord{},
		&gormDNSZoneRecord{},
		&gormDNSRecordRecord{},
		&gormPublicMappingRecord{},
		&gormSecurityGroupRecord{},
		&gormSecurityRuleRecord{},
		&gormOperatorRecord{},
		&gormOperatorSessionRecord{},
		&gormAuditEventRecord{},
		&gormRelayNodeRecord{},
		&gormPunchNodeRecord{},
		&gormCustomerPlanRecord{},
		&gormClientDownloadRecord{},
		&gormPlanRecord{},
		&gormProductRecord{},
		&gormOrderRecord{},
		&gormRenewalRecord{},
	); err != nil {
		return err
	}
	if err := s.ensureSingleDeviceSession(); err != nil {
		return err
	}
	if err := s.migrateNetworkMembershipsToDeviceGroups(); err != nil {
		return err
	}
	return s.ensureIndexes()
}

func (s *GormStore) migrateNetworkMembershipsToDeviceGroups() error {
	if s.db.Dialector.Name() != "postgres" {
		return nil
	}
	if err := s.db.Exec(`
		INSERT INTO gorm_network_device_group_reference_records (network_id, group_id, created_at, updated_at)
		SELECT DISTINCT membership.network_id, group_id.value, EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
		FROM gorm_network_device_records membership
		JOIN gorm_network_records network ON network.network_id = membership.network_id
		JOIN gorm_device_group_assignment_records assignment ON assignment.device_id = membership.device_id
		CROSS JOIN LATERAL json_array_elements_text(assignment.group_ids::json) AS group_id(value)
		JOIN gorm_device_group_records device_group ON device_group.group_id = group_id.value AND device_group.user_id = network.owner_id
		ON CONFLICT (network_id, group_id) DO NOTHING
	`).Error; err != nil {
		return err
	}
	return s.db.Exec(`
		DELETE FROM gorm_network_device_records membership
		WHERE NOT EXISTS (
			SELECT 1
			FROM gorm_network_device_group_reference_records reference
			JOIN gorm_device_group_assignment_records assignment ON assignment.device_id = membership.device_id
			WHERE reference.network_id = membership.network_id
			  AND assignment.group_ids::jsonb ? reference.group_id
		)
	`).Error
}

func (s *GormStore) ensureSingleDeviceSession() error {
	if err := s.db.Exec(`
		DELETE FROM gorm_device_session_records
		WHERE session_id IN (
			SELECT session_id
			FROM (
				SELECT session_id,
					ROW_NUMBER() OVER (
						PARTITION BY device_id
						ORDER BY updated_at DESC, created_at DESC, session_id DESC
					) AS row_number
				FROM gorm_device_session_records
			) ranked
			WHERE row_number > 1
		)
	`).Error; err != nil {
		return err
	}
	return s.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uidx_gorm_device_session_records_device
		ON gorm_device_session_records (device_id)
	`).Error
}

func (s *GormStore) ensureIndexes() error {
	for _, spec := range []struct {
		model any
		name  string
	}{
		{model: &gormUserAliasRecord{}, name: "uidx_gorm_user_alias_records_user_alias"},
		{model: &gormNetworkDeviceRecord{}, name: "uidx_gorm_network_device_records_network_device"},
	} {
		if s.db.Migrator().HasIndex(spec.model, spec.name) {
			continue
		}
		if err := s.db.Migrator().CreateIndex(spec.model, spec.name); err != nil {
			return err
		}
	}
	return nil
}
