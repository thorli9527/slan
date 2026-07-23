package service

import (
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func dnsZoneView(item model.DNSZone) DNSZoneView {
	return DNSZoneView{
		ZoneID:    item.ZoneID,
		NetworkID: item.NetworkID,
		Name:      item.Name,
		Status:    item.Status,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func dnsRecordView(item model.DNSRecord) DNSRecordView {
	view := DNSRecordView{
		RecordID:  item.RecordID,
		NetworkID: item.NetworkID,
		ZoneID:    item.ZoneID,
		Name:      item.Name,
		Type:      item.Type,
		Value:     item.Value,
		Port:      normalizeDNSRecordPort(item.Type, item.Port),
		TTL:       item.TTL,
		Status:    "active",
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
	value := strings.TrimSpace(item.Value)
	switch strings.ToUpper(strings.TrimSpace(item.Type)) {
	case "CNAME":
		view.CNAME = value
	default:
		view.TargetDeviceID = value
	}
	return view
}
