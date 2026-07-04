package web

import (
	"strconv"
	"strings"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func networkPayload(view servicepkg.NetworkSummaryView) map[string]any {
	item := view.Network
	return map[string]any{
		"networkId":        item.NetworkID,
		"workspaceId":      item.NetworkID,
		"ownerUserId":      item.OwnerID,
		"ownerId":          item.OwnerID,
		"name":             item.Name,
		"code":             view.Code,
		"cidr":             item.CIDR,
		"templateKey":      view.TemplateKey,
		"intraGroupPolicy": view.IntraGroupPolicy,
		"default":          view.Default,
		"devices":          view.DeviceCount,
		"members":          view.MemberCount,
		"zone":             view.ZoneName,
		"status":           item.Status,
		"createdAt":        item.CreatedAt,
		"updatedAt":        item.UpdatedAt,
	}
}

func deviceInvitePayload(view servicepkg.DeviceInviteView) map[string]any {
	return map[string]any{
		"inviteId":         view.InviteID,
		"networkId":        view.NetworkID,
		"workspaceId":      view.NetworkID,
		"deviceId":         view.DeviceID,
		"userId":           view.UserID,
		"inviterUserId":    view.InviterUserID,
		"inviterEmail":     view.InviterEmail,
		"inviteCode":       view.InviteCode,
		"status":           view.Status,
		"createdAt":        view.CreatedAt,
		"expiresAt":        view.ExpiresAt,
		"acceptedDeviceId": view.AcceptedDeviceID,
		"acceptedUserId":   view.AcceptedUserID,
		"acceptedAt":       view.AcceptedAt,
	}
}

func networkDevicePayload(view servicepkg.NetworkDeviceView) map[string]any {
	return map[string]any{
		"networkDeviceId":   view.NetworkID + ":" + view.DeviceID,
		"workspaceDeviceId": view.NetworkID + ":" + view.DeviceID,
		"networkId":         view.NetworkID,
		"workspaceId":       view.NetworkID,
		"deviceId":          view.DeviceID,
		"ownerUserId":       view.OwnerUserID,
		"ownerId":           view.OwnerUserID,
		"alias":             view.Alias,
		"deviceAlias":       view.Alias,
		"enabled":           view.Enabled,
		"status":            view.Status,
		"createdAt":         view.CreatedAt,
		"updatedAt":         view.UpdatedAt,
	}
}

func dnsZonePayload(view servicepkg.DNSZoneView) map[string]any {
	return map[string]any{
		"zoneId":       view.ZoneID,
		"networkId":    view.NetworkID,
		"workspaceId":  view.NetworkID,
		"name":         view.Name,
		"zoneName":     view.Name,
		"exposeGlobal": view.ExposeGlobal,
		"status":       view.Status,
		"createdAt":    view.CreatedAt,
		"updatedAt":    view.UpdatedAt,
	}
}

func dnsRecordPayload(view servicepkg.DNSRecordView, zoneName string) map[string]any {
	fqdn := strings.TrimSpace(view.Name)
	zoneName = strings.TrimSpace(zoneName)
	if fqdn != "" && zoneName != "" && !strings.HasSuffix(strings.ToLower(fqdn), "."+strings.ToLower(zoneName)) && !strings.EqualFold(fqdn, zoneName) {
		fqdn = fqdn + "." + zoneName
	}
	return map[string]any{
		"recordId":       view.RecordID,
		"zoneId":         view.ZoneID,
		"networkId":      view.NetworkID,
		"workspaceId":    view.NetworkID,
		"name":           view.Name,
		"zoneName":       zoneName,
		"fqdn":           fqdn,
		"type":           view.Type,
		"recordType":     view.Type,
		"value":          view.Value,
		"targetDeviceId": view.TargetDeviceID,
		"targetIp":       view.TargetIP,
		"cname":          view.CNAME,
		"port":           view.Port,
		"ttl":            view.TTL,
		"status":         view.Status,
		"createdAt":      view.CreatedAt,
		"updatedAt":      view.UpdatedAt,
	}
}

func dnsRecordPayloads(items []servicepkg.DNSRecordView, zoneNames map[string]string) []map[string]any {
	return mapDNSRecordPayloads(items, func(item servicepkg.DNSRecordView) string {
		return zoneNames[item.ZoneID]
	})
}

func mapDNSRecordPayloads(items []servicepkg.DNSRecordView, zoneName func(servicepkg.DNSRecordView) string) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, dnsRecordPayload(item, zoneName(item)))
	}
	return payloads
}

func publicMappingPayload(view servicepkg.PublicMappingView) map[string]any {
	return map[string]any{
		"mappingId":         view.MappingID,
		"networkId":         view.NetworkID,
		"workspaceId":       view.NetworkID,
		"name":              view.Name,
		"alias":             view.Name,
		"publicDomain":      firstNonEmpty(view.PublicDomain, view.Name),
		"sourceRecord":      firstNonEmpty(view.SourceRecord, view.DeviceID, view.InternalIP, view.Name),
		"deviceId":          view.DeviceID,
		"internalIp":        view.InternalIP,
		"targetType":        view.TargetType,
		"protocol":          view.Protocol,
		"internalPort":      view.InternalPort,
		"internalPortValue": view.InternalPort,
		"port":              formatInt(view.InternalPort),
		"externalPortValue": view.ExternalPort,
		"externalPort":      formatInt(view.ExternalPort),
		"accessMode":        view.AccessMode,
		"tlsMode":           view.TLSMode,
		"status":            view.Status,
		"createdAt":         view.CreatedAt,
		"updatedAt":         view.UpdatedAt,
	}
}

func securityGroupPayload(view servicepkg.SecurityGroupView) map[string]any {
	return map[string]any{
		"securityGroupId": view.SecurityGroupID,
		"networkId":       view.NetworkID,
		"workspaceId":     view.NetworkID,
		"name":            view.Name,
		"description":     view.Description,
		"createdAt":       view.CreatedAt,
		"updatedAt":       view.UpdatedAt,
	}
}

func securityRulePayload(view servicepkg.SecurityRuleView) map[string]any {
	return map[string]any{
		"ruleId":          view.RuleID,
		"securityGroupId": view.SecurityGroupID,
		"port":            view.PortRange,
		"direction":       view.Direction,
		"priority":        view.Priority,
		"action":          view.Action,
		"protocol":        view.Protocol,
		"portRange":       view.PortRange,
		"portFrom":        view.PortFrom,
		"portTo":          view.PortTo,
		"peerType":        view.PeerType,
		"peerValue":       view.PeerValue,
		"subjectType":     view.PeerType,
		"subjectValue":    view.PeerValue,
		"description":     view.Description,
		"enabled":         view.Enabled,
		"createdAt":       view.CreatedAt,
		"updatedAt":       view.UpdatedAt,
	}
}

func formatInt(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}
