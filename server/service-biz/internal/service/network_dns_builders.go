package service

import "github.com/slan/service-biz/internal/model"

func dnsZoneViews(items []model.DNSZone) []DNSZoneView {
	views := make([]DNSZoneView, 0, len(items))
	for _, item := range items {
		views = append(views, dnsZoneView(item))
	}
	return views
}

func dnsRecordViews(items []model.DNSRecord) []DNSRecordView {
	views := make([]DNSRecordView, 0, len(items))
	for _, item := range items {
		views = append(views, dnsRecordView(item))
	}
	return views
}
