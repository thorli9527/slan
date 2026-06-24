package service

import "context"

type NetworkDNSUseCase interface {
	ListDNSZones(ctx context.Context, networkID string) ([]DNSZoneView, error)
	AddDNSZone(ctx context.Context, input CreateDNSZoneInput) (DNSZoneView, error)
	UpdateDNSZone(ctx context.Context, input UpdateDNSZoneInput) (DNSZoneView, error)
	DeleteDNSZone(ctx context.Context, input DeleteDNSZoneInput) error
	ListDNSRecords(ctx context.Context, networkID string) ([]DNSRecordView, error)
	AddDNSRecord(ctx context.Context, input CreateDNSRecordInput) (DNSRecordView, error)
	UpdateDNSRecord(ctx context.Context, input UpdateDNSRecordInput) (DNSRecordView, error)
	DeleteDNSRecord(ctx context.Context, input DeleteDNSRecordInput) error
}
