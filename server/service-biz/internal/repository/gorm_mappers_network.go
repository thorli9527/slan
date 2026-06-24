package repository

import "github.com/slan/service-biz/internal/model"

func networkRecordFromModel(item model.Network) gormNetworkRecord {
	return gormNetworkRecord{
		NetworkID:        item.NetworkID,
		OwnerID:          item.OwnerID,
		Name:             item.Name,
		CIDR:             item.CIDR,
		Code:             item.Code,
		TemplateKey:      item.TemplateKey,
		IntraGroupPolicy: item.IntraGroupPolicy,
		Default:          item.Default,
		Status:           item.Status,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func (r gormNetworkRecord) model() model.Network {
	return model.Network{
		NetworkID:        r.NetworkID,
		OwnerID:          r.OwnerID,
		Name:             r.Name,
		CIDR:             r.CIDR,
		Code:             r.Code,
		TemplateKey:      r.TemplateKey,
		IntraGroupPolicy: r.IntraGroupPolicy,
		Default:          r.Default,
		Status:           r.Status,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

func networkDeviceRecordFromModel(item model.NetworkDevice) gormNetworkDeviceRecord {
	return gormNetworkDeviceRecord{
		NetworkID:       item.NetworkID,
		DeviceID:        item.DeviceID,
		Enabled:         item.Enabled,
		Status:          item.Status,
		Endpoints:       jsonDeviceEndpoints(item.Endpoints),
		NATType:         item.NATType,
		ActivePath:      item.ActivePath,
		PathObservedAt:  item.PathObservedAt,
		RelayTransport:  item.RelayTransport,
		RelayEndpoint:   item.RelayEndpoint,
		DerpNodeID:      item.DerpNodeID,
		PeerNodeID:      item.PeerNodeID,
		PathScore:       item.PathScore,
		ObservedRttMs:   item.ObservedRttMs,
		PacketLossPpm:   item.PacketLossPpm,
		RelayMtu:        item.RelayMtu,
		MaxFramePayload: item.MaxFramePayload,
		TicketExpiresAt: item.TicketExpiresAt,
		TicketRenewDue:  item.TicketRenewDue,
		PathDowngrades:  item.PathDowngrades,
		PathUpgrades:    item.PathUpgrades,
		LastPathChange:  item.LastPathChange,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func (r gormNetworkDeviceRecord) model() model.NetworkDevice {
	return model.NetworkDevice{
		NetworkID:       r.NetworkID,
		DeviceID:        r.DeviceID,
		Enabled:         r.Enabled,
		Status:          r.Status,
		Endpoints:       []model.DeviceEndpoint(r.Endpoints),
		NATType:         r.NATType,
		ActivePath:      r.ActivePath,
		PathObservedAt:  r.PathObservedAt,
		RelayTransport:  r.RelayTransport,
		RelayEndpoint:   r.RelayEndpoint,
		DerpNodeID:      r.DerpNodeID,
		PeerNodeID:      r.PeerNodeID,
		PathScore:       r.PathScore,
		ObservedRttMs:   r.ObservedRttMs,
		PacketLossPpm:   r.PacketLossPpm,
		RelayMtu:        r.RelayMtu,
		MaxFramePayload: r.MaxFramePayload,
		TicketExpiresAt: r.TicketExpiresAt,
		TicketRenewDue:  r.TicketRenewDue,
		PathDowngrades:  r.PathDowngrades,
		PathUpgrades:    r.PathUpgrades,
		LastPathChange:  r.LastPathChange,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func deviceInviteRecordFromModel(item model.DeviceInvite) gormDeviceInviteRecord {
	return gormDeviceInviteRecord{
		InviteID:      item.InviteID,
		InviteCode:    item.InviteCode,
		InviterUserID: item.InviterUserID,
		NetworkID:     item.NetworkID,
		DeviceID:      item.DeviceID,
		UserID:        item.UserID,
		Status:        item.Status,
		CreatedAt:     item.CreatedAt,
		ExpiresAt:     item.ExpiresAt,
		AcceptedAt:    item.AcceptedAt,
	}
}

func (r gormDeviceInviteRecord) model() model.DeviceInvite {
	return model.DeviceInvite{
		InviteID:      r.InviteID,
		InviteCode:    r.InviteCode,
		InviterUserID: r.InviterUserID,
		NetworkID:     r.NetworkID,
		DeviceID:      r.DeviceID,
		UserID:        r.UserID,
		Status:        r.Status,
		CreatedAt:     r.CreatedAt,
		ExpiresAt:     r.ExpiresAt,
		AcceptedAt:    r.AcceptedAt,
	}
}

func dnsZoneRecordFromModel(item model.DNSZone) gormDNSZoneRecord {
	return gormDNSZoneRecord{
		ZoneID:       item.ZoneID,
		NetworkID:    item.NetworkID,
		Name:         item.Name,
		ExposeGlobal: item.ExposeGlobal,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func (r gormDNSZoneRecord) model() model.DNSZone {
	return model.DNSZone{
		ZoneID:       r.ZoneID,
		NetworkID:    r.NetworkID,
		Name:         r.Name,
		ExposeGlobal: r.ExposeGlobal,
		Status:       r.Status,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func dnsRecordRecordFromModel(item model.DNSRecord) gormDNSRecordRecord {
	return gormDNSRecordRecord{
		RecordID:  item.RecordID,
		NetworkID: item.NetworkID,
		ZoneID:    item.ZoneID,
		Name:      item.Name,
		Type:      item.Type,
		Value:     item.Value,
		Port:      item.Port,
		TTL:       item.TTL,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func (r gormDNSRecordRecord) model() model.DNSRecord {
	return model.DNSRecord{
		RecordID:  r.RecordID,
		NetworkID: r.NetworkID,
		ZoneID:    r.ZoneID,
		Name:      r.Name,
		Type:      r.Type,
		Value:     r.Value,
		Port:      r.Port,
		TTL:       r.TTL,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func publicMappingRecordFromModel(item model.PublicMapping) gormPublicMappingRecord {
	return gormPublicMappingRecord{
		MappingID:    item.MappingID,
		NetworkID:    item.NetworkID,
		Name:         item.Name,
		PublicDomain: item.PublicDomain,
		SourceRecord: item.SourceRecord,
		DeviceID:     item.DeviceID,
		Protocol:     item.Protocol,
		InternalIP:   item.InternalIP,
		InternalPort: item.InternalPort,
		ExternalPort: item.ExternalPort,
		AccessMode:   item.AccessMode,
		TLSMode:      item.TLSMode,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func (r gormPublicMappingRecord) model() model.PublicMapping {
	return model.PublicMapping{
		MappingID:    r.MappingID,
		NetworkID:    r.NetworkID,
		Name:         r.Name,
		PublicDomain: r.PublicDomain,
		SourceRecord: r.SourceRecord,
		DeviceID:     r.DeviceID,
		Protocol:     r.Protocol,
		InternalIP:   r.InternalIP,
		InternalPort: r.InternalPort,
		ExternalPort: r.ExternalPort,
		AccessMode:   r.AccessMode,
		TLSMode:      r.TLSMode,
		Status:       r.Status,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func securityGroupRecordFromModel(item model.SecurityGroup) gormSecurityGroupRecord {
	return gormSecurityGroupRecord{
		SecurityGroupID: item.SecurityGroupID,
		NetworkID:       item.NetworkID,
		Name:            item.Name,
		Description:     item.Description,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func (r gormSecurityGroupRecord) model() model.SecurityGroup {
	return model.SecurityGroup{
		SecurityGroupID: r.SecurityGroupID,
		NetworkID:       r.NetworkID,
		Name:            r.Name,
		Description:     r.Description,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func securityRuleRecordFromModel(item model.SecurityRule) gormSecurityRuleRecord {
	return gormSecurityRuleRecord{
		RuleID:          item.RuleID,
		SecurityGroupID: item.SecurityGroupID,
		Direction:       item.Direction,
		Protocol:        item.Protocol,
		PortRange:       item.PortRange,
		CIDR:            item.CIDR,
		Action:          item.Action,
		Priority:        item.Priority,
		Description:     item.Description,
		Enabled:         item.Enabled,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func (r gormSecurityRuleRecord) model() model.SecurityRule {
	return model.SecurityRule{
		RuleID:          r.RuleID,
		SecurityGroupID: r.SecurityGroupID,
		Direction:       r.Direction,
		Protocol:        r.Protocol,
		PortRange:       r.PortRange,
		CIDR:            r.CIDR,
		Action:          r.Action,
		Priority:        r.Priority,
		Description:     r.Description,
		Enabled:         r.Enabled,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}
