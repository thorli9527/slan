package service

import (
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func dnsZoneView(item model.DNSZone) DNSZoneView {
	return DNSZoneView{
		ZoneID:       item.ZoneID,
		NetworkID:    item.NetworkID,
		Name:         item.Name,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
		ExposeGlobal: item.ExposeGlobal,
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
		Port:      item.Port,
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
		if strings.Contains(value, ".") {
			view.TargetIP = value
		} else {
			view.TargetDeviceID = value
		}
	}
	return view
}
