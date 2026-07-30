package repository

import "github.com/slan/service-biz/internal/model"

func (s *GormStore) migrate() error {
	if s.db.Migrator().HasTable(&gormDeviceGroupAssignmentRecord{}) {
		if err := s.ensureUserScopedDeviceGroupAssignments(); err != nil {
			return err
		}
	}
	if err := s.db.AutoMigrate(
		&gormCounter{},
		&gormUserRecord{},
		&gormUserSessionRecord{},
		&gormConsoleLoginKeyRecord{},
		&gormUserAliasRecord{},
		&gormDeviceRecord{},
		&gormDeviceUserRelationRecord{},
		&gormDeviceLoginRecord{},
		&gormDeviceSessionRecord{},
		&gormBootstrapKeyRecord{},
		&gormDeviceGroupRecord{},
		&gormDeviceGroupAssignmentRecord{},
		&gormNetworkRecord{},
		&gormNetworkConfigVersionRecord{},
		&gormNetworkEventDeliveryRecord{},
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
	if s.db.Migrator().HasColumn(&gormDNSZoneRecord{}, "expose_global") {
		if err := s.db.Migrator().DropColumn(&gormDNSZoneRecord{}, "expose_global"); err != nil {
			return err
		}
	}
	for _, column := range []string{"code", "template_key"} {
		if s.db.Migrator().HasColumn(&gormNetworkRecord{}, column) {
			if err := s.db.Migrator().DropColumn(&gormNetworkRecord{}, column); err != nil {
				return err
			}
		}
	}
	if err := s.db.Model(&gormDNSRecordRecord{}).
		Where("port <> ? AND UPPER(\"type\") <> ?", "", "SRV").
		Update("port", "").Error; err != nil {
		return err
	}
	if err := s.migrateDeviceOwnershipRelations(); err != nil {
		return err
	}
	if err := s.ensureSingleActiveDeviceOwner(); err != nil {
		return err
	}
	if err := s.ensureSingleDeviceSession(); err != nil {
		return err
	}
	if err := s.ensureSingleUserSessionPerClient(); err != nil {
		return err
	}
	if err := s.enforceWebSessionShortPolicy(); err != nil {
		return err
	}
	if err := s.ensureUserScopedDeviceGroupAssignments(); err != nil {
		return err
	}
	if err := s.migrateNetworkMembershipsToDeviceGroups(); err != nil {
		return err
	}
	return s.ensureIndexes()
}

func (s *GormStore) ensureUserScopedDeviceGroupAssignments() error {
	if s.db.Dialector.Name() != "postgres" {
		return nil
	}
	return s.db.Exec(`
		DO $$
		DECLARE
			primary_key_name text;
			primary_key_columns text[];
		BEGIN
			SELECT constraint_row.conname,
			       array_agg(attribute_row.attname ORDER BY key_column.ordinality)
			INTO primary_key_name, primary_key_columns
			FROM pg_constraint constraint_row
			JOIN unnest(constraint_row.conkey) WITH ORDINALITY AS key_column(attnum, ordinality) ON true
			JOIN pg_attribute attribute_row
			  ON attribute_row.attrelid = constraint_row.conrelid
			 AND attribute_row.attnum = key_column.attnum
			WHERE constraint_row.contype = 'p'
			  AND constraint_row.conrelid = 'gorm_device_group_assignment_records'::regclass
			GROUP BY constraint_row.conname;

			IF primary_key_columns IS DISTINCT FROM ARRAY['device_id', 'user_id']::text[] THEN
				IF primary_key_name IS NOT NULL THEN
					EXECUTE format(
						'ALTER TABLE gorm_device_group_assignment_records DROP CONSTRAINT %I',
						primary_key_name
					);
				END IF;
				ALTER TABLE gorm_device_group_assignment_records
					ADD CONSTRAINT gorm_device_group_assignment_records_pkey
					PRIMARY KEY (device_id, user_id);
			END IF;
		END $$;
	`).Error
}

func (s *GormStore) ensureSingleActiveDeviceOwner() error {
	return s.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uidx_device_user_relation_active_owner
		ON gorm_device_user_relation_records (device_id)
		WHERE role = 'owner' AND status = 'active'
	`).Error
}

func (s *GormStore) migrateDeviceOwnershipRelations() error {
	if !s.db.Migrator().HasColumn("gorm_device_records", "owner_id") {
		return nil
	}
	type legacyDeviceOwner struct {
		DeviceID  string
		OwnerID   string
		CreatedAt int64
		UpdatedAt int64
	}
	var owners []legacyDeviceOwner
	if err := s.db.Table("gorm_device_records").Select("device_id, owner_id, created_at, updated_at").Where("owner_id <> ''").Scan(&owners).Error; err != nil {
		return err
	}
	for _, owner := range owners {
		relation := gormDeviceUserRelationRecord{
			RelationID: deviceUserRelationID(owner.DeviceID, owner.OwnerID),
			DeviceID:   owner.DeviceID,
			UserID:     owner.OwnerID,
			Role:       model.DeviceRelationRoleOwner,
			SourceType: "registration",
			Status:     model.DeviceRelationStatusActive,
			CreatedBy:  owner.OwnerID,
			CreatedAt:  owner.CreatedAt,
			UpdatedAt:  owner.UpdatedAt,
		}
		if err := upsertByColumns(s.db, &relation, []string{"device_id", "user_id"}, []string{"relation_id", "role", "source_type", "source_id", "status", "created_by", "created_at", "updated_at", "revoked_by", "revoked_at"}); err != nil {
			return err
		}
	}
	return s.db.Migrator().DropColumn("gorm_device_records", "owner_id")
}

func (s *GormStore) migrateNetworkMembershipsToDeviceGroups() error {
	if s.db.Dialector.Name() != "postgres" {
		return nil
	}
	// Backfill group references without deleting existing memberships. Membership
	// removal belongs to the device-group use case, which also publishes the
	// corresponding network snapshot and client control event. Deleting rows here
	// on every service start silently disconnects legacy or temporarily unmapped
	// devices and leaves enabled clients with stale routes.
	return s.db.Exec(`
		INSERT INTO gorm_network_device_group_reference_records (network_id, group_id, created_at, updated_at)
		SELECT DISTINCT membership.network_id, group_id.value, EXTRACT(EPOCH FROM NOW())::BIGINT, EXTRACT(EPOCH FROM NOW())::BIGINT
		FROM gorm_network_device_records membership
		JOIN gorm_network_records network ON network.network_id = membership.network_id
		JOIN gorm_device_group_assignment_records assignment
		  ON assignment.device_id = membership.device_id
		 AND assignment.user_id = network.owner_id
		CROSS JOIN LATERAL json_array_elements_text(assignment.group_ids::json) AS group_id(value)
		JOIN gorm_device_group_records device_group ON device_group.group_id = group_id.value AND device_group.user_id = network.owner_id
		ON CONFLICT (network_id, group_id) DO NOTHING
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

func (s *GormStore) ensureSingleUserSessionPerClient() error {
	if err := s.db.Exec(`DROP INDEX IF EXISTS uidx_gorm_user_session_records_user_client`).Error; err != nil {
		return err
	}
	if err := s.db.Exec(`UPDATE gorm_user_session_records SET device_id = '' WHERE device_id IS NULL`).Error; err != nil {
		return err
	}
	if err := s.db.Exec(`
		DELETE FROM gorm_user_session_records
		WHERE client_type IS NULL OR client_type = ''
	`).Error; err != nil {
		return err
	}
	if err := s.db.Exec(`
		DELETE FROM gorm_user_session_records
		WHERE session_id IN (
			SELECT session_id
			FROM (
				SELECT session_id,
					ROW_NUMBER() OVER (
						PARTITION BY user_id, client_type, device_id
						ORDER BY updated_at DESC, created_at DESC, session_id DESC
					) AS row_number
				FROM gorm_user_session_records
			) ranked
			WHERE row_number > 1
		)
	`).Error; err != nil {
		return err
	}
	return s.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS uidx_gorm_user_session_records_user_client_device
		ON gorm_user_session_records (user_id, client_type, device_id)
	`).Error
}

func (s *GormStore) enforceWebSessionShortPolicy() error {
	return s.db.Exec(`
		UPDATE gorm_user_session_records
		SET session_mode = 'short',
			refresh_expiry = LEAST(
				refresh_expiry,
				EXTRACT(EPOCH FROM NOW())::bigint + 604800
			)
		WHERE client_type = 'web'
			AND (
				session_mode <> 'short'
				OR refresh_expiry > EXTRACT(EPOCH FROM NOW())::bigint + 604800
			)
	`).Error
}

func (s *GormStore) ensureIndexes() error {
	for _, spec := range []struct {
		model any
		name  string
	}{
		{model: &gormUserAliasRecord{}, name: "uidx_gorm_user_alias_records_user_alias"},
		{model: &gormNetworkDeviceRecord{}, name: "uidx_gorm_network_device_records_network_device"},
		{model: &gormDeviceUserRelationRecord{}, name: "uidx_device_user_relation"},
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
