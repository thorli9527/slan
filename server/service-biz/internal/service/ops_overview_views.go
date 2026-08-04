package service

import "github.com/slan/service-biz/internal/model"

func opsAuditEventView(item model.AuditEvent) OpsAuditEventView {
	return OpsAuditEventView{
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
