package repository

import "github.com/slan/service-biz/internal/model"

func auditEventRecordFromModel(item model.AuditEvent) gormAuditEventRecord {
	return gormAuditEventRecord{
		EventID:      item.EventID,
		ActorType:    item.ActorType,
		ActorID:      item.ActorID,
		Action:       item.Action,
		ResourceType: item.ResourceType,
		ResourceID:   item.ResourceID,
		Status:       item.Status,
		RemoteIP:     item.RemoteIP,
		Detail:       item.Detail,
		CreatedAt:    item.CreatedAt,
	}
}

func (r gormAuditEventRecord) model() model.AuditEvent {
	return model.AuditEvent{
		EventID:      r.EventID,
		ActorType:    r.ActorType,
		ActorID:      r.ActorID,
		Action:       r.Action,
		ResourceType: r.ResourceType,
		ResourceID:   r.ResourceID,
		Status:       r.Status,
		RemoteIP:     r.RemoteIP,
		Detail:       r.Detail,
		CreatedAt:    r.CreatedAt,
	}
}

func relayNodeRecordFromModel(item model.RelayNode) gormRelayNodeRecord {
	return gormRelayNodeRecord{
		NodeID:                   item.NodeID,
		Name:                     item.Name,
		Region:                   item.Region,
		Endpoint:                 item.Endpoint,
		Transport:                item.Transport,
		Priority:                 item.Priority,
		TicketKeySource:          item.TicketKeySource,
		TicketKeyRingID:          item.TicketKeyRingID,
		TicketSigningConfigured:  item.TicketSigningConfigured,
		TicketKeyRingConfigured:  item.TicketKeyRingConfigured,
		TicketEffectiveKeyCount:  item.TicketEffectiveKeyCount,
		TicketRotationReady:      item.TicketRotationReady,
		TicketAcceptsDevFallback: item.TicketAcceptsDevFallback,
		MaxBandwidthMbps:         item.MaxBandwidthMbps,
		MonthlyTrafficGb:         item.MonthlyTrafficGB,
		UsedTrafficGb:            item.UsedTrafficGB,
		MaxSessions:              item.MaxSessions,
		ActiveSessions:           item.ActiveSessions,
		Status:                   item.Status,
		Health:                   item.Health,
		CreatedAt:                item.CreatedAt,
		UpdatedAt:                item.UpdatedAt,
	}
}

func (r gormRelayNodeRecord) model() model.RelayNode {
	return model.RelayNode{
		NodeID:                   r.NodeID,
		Name:                     r.Name,
		Region:                   r.Region,
		Endpoint:                 r.Endpoint,
		Transport:                r.Transport,
		Priority:                 r.Priority,
		TicketKeySource:          r.TicketKeySource,
		TicketKeyRingID:          r.TicketKeyRingID,
		TicketSigningConfigured:  r.TicketSigningConfigured,
		TicketKeyRingConfigured:  r.TicketKeyRingConfigured,
		TicketEffectiveKeyCount:  r.TicketEffectiveKeyCount,
		TicketRotationReady:      r.TicketRotationReady,
		TicketAcceptsDevFallback: r.TicketAcceptsDevFallback,
		MaxBandwidthMbps:         r.MaxBandwidthMbps,
		MonthlyTrafficGB:         r.MonthlyTrafficGb,
		UsedTrafficGB:            r.UsedTrafficGb,
		MaxSessions:              r.MaxSessions,
		ActiveSessions:           r.ActiveSessions,
		Status:                   r.Status,
		Health:                   r.Health,
		CreatedAt:                r.CreatedAt,
		UpdatedAt:                r.UpdatedAt,
	}
}

func punchNodeRecordFromModel(item model.PunchNode) gormPunchNodeRecord {
	return gormPunchNodeRecord{
		NodeID:         item.NodeID,
		Name:           item.Name,
		Endpoint:       item.Endpoint,
		MaxSessions:    item.MaxSessions,
		ActiveSessions: item.ActiveSessions,
		Status:         item.Status,
		Health:         item.Health,
		Priority:       item.Priority,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func (r gormPunchNodeRecord) model() model.PunchNode {
	return model.PunchNode{
		NodeID:         r.NodeID,
		Name:           r.Name,
		Endpoint:       r.Endpoint,
		MaxSessions:    r.MaxSessions,
		ActiveSessions: r.ActiveSessions,
		Status:         r.Status,
		Health:         r.Health,
		Priority:       r.Priority,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func serverNodeRecordFromModel(item model.ServerNode) gormServerNodeRecord {
	return gormServerNodeRecord{
		NodeID: item.NodeID, Name: item.Name, Host: item.Host, SSHPort: item.SSHPort,
		SSHUsername: item.SSHUsername, SSHPasswordCiphertext: item.SSHPasswordCiphertext,
		SSHHostKeyFingerprint: item.SSHHostKeyFingerprint, RelayUDPPort: item.RelayUDPPort,
		RelayAdminPort: item.RelayAdminPort, RelayTCPPort: item.RelayTCPPort, PunchUDPPort: item.PunchUDPPort,
		PunchHTTPPort: item.PunchHTTPPort, RelayNodeID: item.RelayNodeID, PunchNodeID: item.PunchNodeID,
		APIProxyPort: item.APIProxyPort, MQTTProxyPort: item.MQTTProxyPort,
		RelayEnabled: item.RelayEnabled, PunchEnabled: item.PunchEnabled, ProxyEnabled: item.ProxyEnabled,
		RelayTCPNodeID: item.RelayTCPNodeID,
		DeployStatus:   item.DeployStatus, LastDeployError: item.LastDeployError,
		LastDeployedAt: item.LastDeployedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (r gormServerNodeRecord) model() model.ServerNode {
	return model.ServerNode{
		NodeID: r.NodeID, Name: r.Name, Host: r.Host, SSHPort: r.SSHPort,
		SSHUsername: r.SSHUsername, SSHPasswordCiphertext: r.SSHPasswordCiphertext,
		SSHHostKeyFingerprint: r.SSHHostKeyFingerprint, RelayUDPPort: r.RelayUDPPort,
		RelayAdminPort: r.RelayAdminPort, RelayTCPPort: r.RelayTCPPort, PunchUDPPort: r.PunchUDPPort,
		PunchHTTPPort: r.PunchHTTPPort, RelayNodeID: r.RelayNodeID, PunchNodeID: r.PunchNodeID,
		APIProxyPort: r.APIProxyPort, MQTTProxyPort: r.MQTTProxyPort,
		RelayEnabled: r.RelayEnabled, PunchEnabled: r.PunchEnabled, ProxyEnabled: r.ProxyEnabled,
		RelayTCPNodeID: r.RelayTCPNodeID,
		DeployStatus:   r.DeployStatus, LastDeployError: r.LastDeployError,
		LastDeployedAt: r.LastDeployedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
