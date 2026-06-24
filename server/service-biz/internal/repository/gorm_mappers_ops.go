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
		NodeID:           item.NodeID,
		Name:             item.Name,
		Region:           item.Region,
		Endpoint:         item.Endpoint,
		Transport:        item.Transport,
		Priority:         item.Priority,
		TicketKeySource:          item.TicketKeySource,
		TicketKeyRingID:          item.TicketKeyRingID,
		TicketSigningConfigured:  item.TicketSigningConfigured,
		TicketKeyRingConfigured:  item.TicketKeyRingConfigured,
		TicketEffectiveKeyCount:  item.TicketEffectiveKeyCount,
		TicketRotationReady:      item.TicketRotationReady,
		TicketAcceptsDevFallback: item.TicketAcceptsDevFallback,
		MaxBandwidthMbps: item.MaxBandwidthMbps,
		MonthlyTrafficGb: item.MonthlyTrafficGB,
		UsedTrafficGb:    item.UsedTrafficGB,
		MaxSessions:      item.MaxSessions,
		ActiveSessions:   item.ActiveSessions,
		Status:           item.Status,
		Health:           item.Health,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func (r gormRelayNodeRecord) model() model.RelayNode {
	return model.RelayNode{
		NodeID:           r.NodeID,
		Name:             r.Name,
		Region:           r.Region,
		Endpoint:         r.Endpoint,
		Transport:        r.Transport,
		Priority:         r.Priority,
		TicketKeySource:          r.TicketKeySource,
		TicketKeyRingID:          r.TicketKeyRingID,
		TicketSigningConfigured:  r.TicketSigningConfigured,
		TicketKeyRingConfigured:  r.TicketKeyRingConfigured,
		TicketEffectiveKeyCount:  r.TicketEffectiveKeyCount,
		TicketRotationReady:      r.TicketRotationReady,
		TicketAcceptsDevFallback: r.TicketAcceptsDevFallback,
		MaxBandwidthMbps: r.MaxBandwidthMbps,
		MonthlyTrafficGB: r.MonthlyTrafficGb,
		UsedTrafficGB:    r.UsedTrafficGb,
		MaxSessions:      r.MaxSessions,
		ActiveSessions:   r.ActiveSessions,
		Status:           r.Status,
		Health:           r.Health,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
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

func clientDownloadRecordFromModel(item model.ClientDownload) gormClientDownloadRecord {
	return gormClientDownloadRecord{
		DownloadID:   item.DownloadID,
		Name:         item.Name,
		Platform:     item.Platform,
		Version:      item.Version,
		Arch:         item.Arch,
		Channel:      item.Channel,
		URL:          item.URL,
		SHA256:       item.SHA256,
		ReleaseNotes: item.ReleaseNotes,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func (r gormClientDownloadRecord) model() model.ClientDownload {
	return model.ClientDownload{
		DownloadID:   r.DownloadID,
		Name:         r.Name,
		Platform:     r.Platform,
		Version:      r.Version,
		Arch:         r.Arch,
		Channel:      r.Channel,
		URL:          r.URL,
		SHA256:       r.SHA256,
		ReleaseNotes: r.ReleaseNotes,
		Status:       r.Status,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func planRecordFromModel(item model.Plan) gormPlanRecord {
	return gormPlanRecord{
		PlanCode:           item.PlanCode,
		Name:               item.Name,
		DeviceLimit:        item.DeviceLimit,
		InvitedDeviceLimit: item.InvitedDeviceLimit,
		TotalDeviceLimit:   item.TotalDeviceLimit,
		RelayMonthlyGb:     item.RelayMonthlyGB,
		RelayBandwidthMbps: item.RelayBandwidthMbps,
		RelayThrottleMbps:  item.RelayThrottleMbps,
		P2PUnlimited:       item.P2PUnlimited,
		CustomDomain:       item.CustomDomain,
		Acl:                item.ACL,
		DedicatedRelay:     item.DedicatedRelay,
		AuditLog:           item.AuditLog,
		ApiAccess:          item.APIAccess,
		MonthlyPrice:       item.MonthlyPrice,
		YearlyPrice:        item.YearlyPrice,
		Status:             item.Status,
		UpdatedAt:          item.UpdatedAt,
	}
}

func (r gormPlanRecord) model() model.Plan {
	return model.Plan{
		PlanCode:           r.PlanCode,
		Name:               r.Name,
		DeviceLimit:        r.DeviceLimit,
		InvitedDeviceLimit: r.InvitedDeviceLimit,
		TotalDeviceLimit:   r.TotalDeviceLimit,
		RelayMonthlyGB:     r.RelayMonthlyGb,
		RelayBandwidthMbps: r.RelayBandwidthMbps,
		RelayThrottleMbps:  r.RelayThrottleMbps,
		P2PUnlimited:       r.P2PUnlimited,
		CustomDomain:       r.CustomDomain,
		ACL:                r.Acl,
		DedicatedRelay:     r.DedicatedRelay,
		AuditLog:           r.AuditLog,
		APIAccess:          r.ApiAccess,
		MonthlyPrice:       r.MonthlyPrice,
		YearlyPrice:        r.YearlyPrice,
		Status:             r.Status,
		UpdatedAt:          r.UpdatedAt,
	}
}

func productRecordFromModel(item model.Product) gormProductRecord {
	return gormProductRecord{
		ProductID:          item.ProductID,
		Name:               item.Name,
		Type:               item.Type,
		PlanCode:           item.PlanCode,
		Period:             item.Period,
		ValidDays:          item.ValidDays,
		RelayTrafficGb:     item.RelayTrafficGB,
		RelayBandwidthMbps: item.RelayBandwidthMbps,
		Price:              item.Price,
		SalePrice:          item.SalePrice,
		Currency:           item.Currency,
		AutoRenew:          item.AutoRenew,
		Status:             item.Status,
		Description:        item.Description,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func (r gormProductRecord) model() model.Product {
	return model.Product{
		ProductID:          r.ProductID,
		Name:               r.Name,
		Type:               r.Type,
		PlanCode:           r.PlanCode,
		Period:             r.Period,
		ValidDays:          r.ValidDays,
		RelayTrafficGB:     r.RelayTrafficGb,
		RelayBandwidthMbps: r.RelayBandwidthMbps,
		Price:              r.Price,
		SalePrice:          r.SalePrice,
		Currency:           r.Currency,
		AutoRenew:          r.AutoRenew,
		Status:             r.Status,
		Description:        r.Description,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

func orderRecordFromModel(item model.Order) gormOrderRecord {
	return gormOrderRecord{
		OrderID:         item.OrderID,
		CustomerID:      item.CustomerID,
		CustomerEmail:   item.CustomerEmail,
		ProductID:       item.ProductID,
		ProductName:     item.ProductName,
		ProductType:     item.ProductType,
		Status:          item.Status,
		Amount:          item.Amount,
		Currency:        item.Currency,
		PayStatus:       item.PayStatus,
		ProvisionStatus: item.ProvisionStatus,
		Channel:         item.Channel,
		PaidAt:          item.PaidAt,
		ValidUntil:      item.ValidUntil,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func (r gormOrderRecord) model() model.Order {
	return model.Order{
		OrderID:         r.OrderID,
		CustomerID:      r.CustomerID,
		CustomerEmail:   r.CustomerEmail,
		ProductID:       r.ProductID,
		ProductName:     r.ProductName,
		ProductType:     r.ProductType,
		Status:          r.Status,
		Amount:          r.Amount,
		Currency:        r.Currency,
		PayStatus:       r.PayStatus,
		ProvisionStatus: r.ProvisionStatus,
		Channel:         r.Channel,
		PaidAt:          r.PaidAt,
		ValidUntil:      r.ValidUntil,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func renewalRecordFromModel(item model.Renewal) gormRenewalRecord {
	return gormRenewalRecord{
		RenewalID:     item.RenewalID,
		OrderID:       item.OrderID,
		CustomerID:    item.CustomerID,
		CustomerEmail: item.CustomerEmail,
		PlanCode:      item.PlanCode,
		Period:        item.Period,
		Amount:        item.Amount,
		Status:        item.Status,
		RenewAt:       item.RenewAt,
		PaidAt:        item.PaidAt,
		Source:        item.Source,
		Operator:      item.Operator,
		UpdatedAt:     item.UpdatedAt,
	}
}

func (r gormRenewalRecord) model() model.Renewal {
	return model.Renewal{
		RenewalID:     r.RenewalID,
		OrderID:       r.OrderID,
		CustomerID:    r.CustomerID,
		CustomerEmail: r.CustomerEmail,
		PlanCode:      r.PlanCode,
		Period:        r.Period,
		Amount:        r.Amount,
		Status:        r.Status,
		RenewAt:       r.RenewAt,
		PaidAt:        r.PaidAt,
		Source:        r.Source,
		Operator:      r.Operator,
		UpdatedAt:     r.UpdatedAt,
	}
}
