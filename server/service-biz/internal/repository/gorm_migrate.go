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
	return s.ensureIndexes()
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
