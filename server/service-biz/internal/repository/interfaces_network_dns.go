package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type NetworkDNSRepository interface {
	ListDNSZones(ctx context.Context, networkID string) ([]model.DNSZone, error)
	GetDNSZone(ctx context.Context, zoneID string) (model.DNSZone, bool, error)
	SaveDNSZone(ctx context.Context, zone model.DNSZone) error
	DeleteDNSZone(ctx context.Context, zoneID string) error
	ListDNSRecords(ctx context.Context, networkID string) ([]model.DNSRecord, error)
	GetDNSRecord(ctx context.Context, recordID string) (model.DNSRecord, bool, error)
	SaveDNSRecord(ctx context.Context, record model.DNSRecord) error
	DeleteDNSRecord(ctx context.Context, recordID string) error
}
