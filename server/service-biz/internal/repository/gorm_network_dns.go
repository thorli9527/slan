package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s *GormStore) ListDNSZones(_ context.Context, networkID string) ([]model.DNSZone, error) {
	return listModels(s.db.Where("network_id = ?", networkID).Order("zone_id asc"), func(row gormDNSZoneRecord) model.DNSZone {
		return row.model()
	})
}

func (s *GormStore) GetDNSZone(_ context.Context, zoneID string) (model.DNSZone, bool, error) {
	return firstModel(s.db.Where("zone_id = ?", zoneID), func(row gormDNSZoneRecord) model.DNSZone {
		return row.model()
	})
}

func (s *GormStore) SaveDNSZone(_ context.Context, zone model.DNSZone) error {
	row := dnsZoneRecordFromModel(zone)
	return upsertByColumns(s.db, &row, []string{"zone_id"}, []string{"network_id", "name", "expose_global", "status", "created_at", "updated_at"})
}

func (s *GormStore) DeleteDNSZone(_ context.Context, zoneID string) error {
	return s.db.Delete(&gormDNSZoneRecord{}, "zone_id = ?", zoneID).Error
}

func (s *GormStore) ListDNSRecords(_ context.Context, networkID string) ([]model.DNSRecord, error) {
	return listModels(s.db.Where("network_id = ?", networkID).Order("record_id asc"), func(row gormDNSRecordRecord) model.DNSRecord {
		return row.model()
	})
}

func (s *GormStore) GetDNSRecord(_ context.Context, recordID string) (model.DNSRecord, bool, error) {
	return firstModel(s.db.Where("record_id = ?", recordID), func(row gormDNSRecordRecord) model.DNSRecord {
		return row.model()
	})
}

func (s *GormStore) SaveDNSRecord(_ context.Context, record model.DNSRecord) error {
	row := dnsRecordRecordFromModel(record)
	return upsertByColumns(s.db, &row, []string{"record_id"}, []string{"network_id", "zone_id", "name", "type", "value", "port", "ttl", "created_at", "updated_at"})
}

func (s *GormStore) DeleteDNSRecord(_ context.Context, recordID string) error {
	return s.db.Delete(&gormDNSRecordRecord{}, "record_id = ?", recordID).Error
}
