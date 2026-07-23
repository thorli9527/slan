package service

import "strings"

func normalizeCreateDNSZoneInput(input CreateDNSZoneInput) CreateDNSZoneInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	return input
}

func normalizeUpdateDNSZoneInput(input UpdateDNSZoneInput) UpdateDNSZoneInput {
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.TrimSpace(input.Status)
	return input
}

func normalizeDNSZoneID(zoneID string) string {
	return strings.TrimSpace(zoneID)
}

func normalizeCreateDNSRecordInput(input CreateDNSRecordInput) CreateDNSRecordInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	input.Value = strings.TrimSpace(input.Value)
	input.Port = normalizeDNSRecordPort(input.Type, input.Port)
	return input
}

func normalizeUpdateDNSRecordInput(input UpdateDNSRecordInput) UpdateDNSRecordInput {
	input.RecordID = strings.TrimSpace(input.RecordID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	input.Value = strings.TrimSpace(input.Value)
	input.Port = normalizeDNSRecordPort(input.Type, input.Port)
	return input
}

func normalizeDNSRecordPort(recordType, port string) string {
	if !strings.EqualFold(strings.TrimSpace(recordType), "SRV") {
		return ""
	}
	return strings.TrimSpace(port)
}

func normalizeDNSRecordID(recordID string) string {
	return strings.TrimSpace(recordID)
}

func normalizeDeleteDNSZoneInput(input DeleteDNSZoneInput) DeleteDNSZoneInput {
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}

func normalizeDeleteDNSRecordInput(input DeleteDNSRecordInput) DeleteDNSRecordInput {
	input.RecordID = strings.TrimSpace(input.RecordID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}
