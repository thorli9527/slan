package repository

func (s *GormStore) migrate() error {
	if err := s.db.AutoMigrate(
		&gormCounter{},
		&gormCustomerRecord{},
		&gormDeviceRecord{},
		&gormDeviceSessionRecord{},
		&gormDeviceCredentialRecord{},
		&gormDeviceGroupRecord{},
		&gormDeviceGroupAssignmentRecord{},
		&gormNetworkRecord{},
		&gormNetworkConfigVersionRecord{},
		&gormNetworkEventDeliveryRecord{},
		&gormNetworkDeviceRecord{},
		&gormNetworkDeviceGroupReferenceRecord{},
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
	); err != nil {
		return err
	}
	if err := s.dropLegacyDeviceCredentialExpiry(); err != nil {
		return err
	}
	if err := s.revokeDuplicateActiveDeviceCredentials(); err != nil {
		return err
	}
	return s.ensureIndexes()
}

func (s *GormStore) dropLegacyDeviceCredentialExpiry() error {
	migrator := s.db.Migrator()
	if !migrator.HasColumn(&gormDeviceCredentialRecord{}, "expires_at") {
		return nil
	}
	return migrator.DropColumn(&gormDeviceCredentialRecord{}, "expires_at")
}

func (s *GormStore) revokeDuplicateActiveDeviceCredentials() error {
	return s.db.Exec(`
		WITH ranked AS (
			SELECT credential_id,
				ROW_NUMBER() OVER (
					PARTITION BY device_id
					ORDER BY created_at DESC, credential_id DESC
				) AS position
			FROM gorm_device_credential_records
			WHERE device_id <> '' AND status = 'active'
		)
		UPDATE gorm_device_credential_records AS credentials
		SET status = 'revoked',
			revoked_at = GREATEST(credentials.updated_at, credentials.created_at),
			updated_at = GREATEST(credentials.updated_at, credentials.created_at)
		FROM ranked
		WHERE credentials.credential_id = ranked.credential_id
			AND ranked.position > 1
	`).Error
}

func (s *GormStore) ensureIndexes() error {
	for _, spec := range []struct {
		model any
		name  string
	}{
		{model: &gormNetworkDeviceRecord{}, name: "uidx_gorm_network_device_records_network_device"},
	} {
		if s.db.Migrator().HasIndex(spec.model, spec.name) {
			continue
		}
		if err := s.db.Migrator().CreateIndex(spec.model, spec.name); err != nil {
			return err
		}
	}
	return s.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uidx_device_credentials_active_device
		ON gorm_device_credential_records (device_id)
		WHERE device_id <> '' AND status = 'active'
	`).Error
}
