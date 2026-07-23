package service

import "github.com/slan/service-biz/internal/model"

func newManagedDNSZone(id string, now int64, input CreateDNSZoneInput) model.DNSZone {
	return model.DNSZone{
		ZoneID:    id,
		NetworkID: input.NetworkID,
		Name:      input.Name,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func applyUpdateDNSZoneInput(item model.DNSZone, input UpdateDNSZoneInput, now int64) model.DNSZone {
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	item.UpdatedAt = now
	return item
}

func newManagedDNSRecord(id string, now int64, input CreateDNSRecordInput) model.DNSRecord {
	return model.DNSRecord{
		RecordID:  id,
		NetworkID: input.NetworkID,
		ZoneID:    input.ZoneID,
		Name:      input.Name,
		Type:      input.Type,
		Value:     input.Value,
		Port:      input.Port,
		TTL:       input.TTL,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func applyUpdateDNSRecordInput(item model.DNSRecord, input UpdateDNSRecordInput, now int64) model.DNSRecord {
	if input.ZoneID != "" {
		item.ZoneID = input.ZoneID
	}
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.Type != "" {
		item.Type = input.Type
	}
	if input.Value != "" {
		item.Value = input.Value
	}
	item.Port = input.Port
	if input.TTL > 0 {
		item.TTL = input.TTL
	}
	item.UpdatedAt = now
	return item
}
