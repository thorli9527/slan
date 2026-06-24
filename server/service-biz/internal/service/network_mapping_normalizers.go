package service

import "strings"

func normalizeCreatePublicMappingInput(input CreatePublicMappingInput) CreatePublicMappingInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	input.PublicDomain = strings.TrimSpace(input.PublicDomain)
	input.SourceRecord = strings.TrimSpace(input.SourceRecord)
	input.Protocol = strings.TrimSpace(input.Protocol)
	input.InternalIP = strings.TrimSpace(input.InternalIP)
	input.AccessMode = strings.TrimSpace(input.AccessMode)
	input.TLSMode = strings.TrimSpace(input.TLSMode)
	if input.AccessMode == "" {
		input.AccessMode = "public"
	}
	if input.TLSMode == "" {
		input.TLSMode = "auto"
	}
	return input
}

func normalizeUpdatePublicMappingInput(input UpdatePublicMappingInput) UpdatePublicMappingInput {
	input.MappingID = strings.TrimSpace(input.MappingID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Name = strings.TrimSpace(input.Name)
	input.PublicDomain = strings.TrimSpace(input.PublicDomain)
	input.SourceRecord = strings.TrimSpace(input.SourceRecord)
	input.Protocol = strings.TrimSpace(input.Protocol)
	input.InternalIP = strings.TrimSpace(input.InternalIP)
	input.AccessMode = strings.TrimSpace(input.AccessMode)
	input.TLSMode = strings.TrimSpace(input.TLSMode)
	input.Status = normalizePublicMappingStatus(strings.TrimSpace(input.Status))
	return input
}

func normalizePublicMappingID(mappingID string) string {
	return strings.TrimSpace(mappingID)
}

func normalizeDeletePublicMappingInput(input DeletePublicMappingInput) DeletePublicMappingInput {
	input.MappingID = strings.TrimSpace(input.MappingID)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	return input
}

func normalizePublicMappingStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", "enabled", "active":
		return "enabled"
	case "disabled", "inactive":
		return "disabled"
	default:
		return status
	}
}
