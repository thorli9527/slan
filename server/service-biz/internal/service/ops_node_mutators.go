package service

import "github.com/slan/service-biz/internal/model"

func newRelayNode(nodeID string, now int64, input UpsertNodeInput) model.RelayNode {
	return model.RelayNode{
		NodeID:           nodeID,
		Name:             input.Name,
		Region:           input.Region,
		Endpoint:         input.Endpoint,
		Transport:        firstNonEmpty(input.Transport, "relay_udp"),
		Priority:         input.Priority,
		MaxBandwidthMbps: input.MaxBandwidthMbps,
		MonthlyTrafficGB: input.MonthlyTrafficGB,
		UsedTrafficGB:    input.UsedTrafficGB,
		MaxSessions:      input.MaxSessions,
		ActiveSessions:   input.ActiveSessions,
		Status:           firstNonEmpty(input.Status, "active"),
		Health:           firstNonEmpty(input.Health, "healthy"),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func mergeRelayNode(current model.RelayNode, input UpsertNodeInput, now int64) model.RelayNode {
	item := newRelayNode(current.NodeID, now, input)
	item.CreatedAt = current.CreatedAt
	if item.Name == "" {
		item.Name = current.Name
	}
	if item.Region == "" {
		item.Region = current.Region
	}
	if item.Endpoint == "" {
		item.Endpoint = current.Endpoint
	}
	if item.Transport == "" {
		item.Transport = current.Transport
	}
	if item.Priority == 0 {
		item.Priority = current.Priority
	}
	if item.TicketKeySource == "" {
		item.TicketKeySource = current.TicketKeySource
	}
	if item.TicketKeyRingID == "" {
		item.TicketKeyRingID = current.TicketKeyRingID
	}
	if !item.TicketSigningConfigured {
		item.TicketSigningConfigured = current.TicketSigningConfigured
	}
	if !item.TicketKeyRingConfigured {
		item.TicketKeyRingConfigured = current.TicketKeyRingConfigured
	}
	if item.TicketEffectiveKeyCount == 0 {
		item.TicketEffectiveKeyCount = current.TicketEffectiveKeyCount
	}
	if !item.TicketRotationReady {
		item.TicketRotationReady = current.TicketRotationReady
	}
	if !item.TicketAcceptsDevFallback {
		item.TicketAcceptsDevFallback = current.TicketAcceptsDevFallback
	}
	if item.MaxBandwidthMbps == 0 {
		item.MaxBandwidthMbps = current.MaxBandwidthMbps
	}
	if item.MonthlyTrafficGB == 0 {
		item.MonthlyTrafficGB = current.MonthlyTrafficGB
	}
	if item.UsedTrafficGB == 0 {
		item.UsedTrafficGB = current.UsedTrafficGB
	}
	if item.MaxSessions == 0 {
		item.MaxSessions = current.MaxSessions
	}
	if item.ActiveSessions == 0 {
		item.ActiveSessions = current.ActiveSessions
	}
	if input.Status == "" {
		item.Status = current.Status
	}
	if input.Health == "" {
		item.Health = current.Health
	}
	return item
}

func newPunchNode(nodeID string, now int64, input UpsertNodeInput) model.PunchNode {
	return model.PunchNode{
		NodeID:         nodeID,
		Name:           input.Name,
		Region:         input.Region,
		Endpoint:       input.Endpoint,
		MaxSessions:    input.MaxSessions,
		ActiveSessions: input.ActiveSessions,
		Status:         firstNonEmpty(input.Status, "active"),
		Health:         firstNonEmpty(input.Health, "healthy"),
		Priority:       input.Priority,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func mergePunchNode(current model.PunchNode, input UpsertNodeInput, now int64) model.PunchNode {
	item := newPunchNode(current.NodeID, now, input)
	item.CreatedAt = current.CreatedAt
	if item.Name == "" {
		item.Name = current.Name
	}
	if item.Region == "" {
		item.Region = current.Region
	}
	if item.Endpoint == "" {
		item.Endpoint = current.Endpoint
	}
	if item.MaxSessions == 0 {
		item.MaxSessions = current.MaxSessions
	}
	if item.ActiveSessions == 0 {
		item.ActiveSessions = current.ActiveSessions
	}
	if input.Status == "" {
		item.Status = current.Status
	}
	if input.Health == "" {
		item.Health = current.Health
	}
	if item.Priority == 0 {
		item.Priority = current.Priority
	}
	return item
}

func applyRelayNodeStatus(item model.RelayNode, status string, now int64) model.RelayNode {
	item.Status = normalizeNodeStatus(status)
	item.UpdatedAt = now
	return item
}

func applyPunchNodeStatus(item model.PunchNode, status string, now int64) model.PunchNode {
	item.Status = normalizeNodeStatus(status)
	item.UpdatedAt = now
	return item
}
