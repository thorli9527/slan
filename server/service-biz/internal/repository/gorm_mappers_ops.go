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
		Region:         item.Region,
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
		Region:         r.Region,
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
