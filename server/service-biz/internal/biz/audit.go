package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) RecordAuditEvent(event AuditEvent) AuditEvent {
	event.ActorType = strings.TrimSpace(event.ActorType)
	event.ActorID = strings.TrimSpace(event.ActorID)
	event.ActorEmail = strings.ToLower(strings.TrimSpace(event.ActorEmail))
	event.Action = strings.TrimSpace(event.Action)
	event.ResourceType = strings.TrimSpace(event.ResourceType)
	event.ResourceID = strings.TrimSpace(event.ResourceID)
	event.Status = strings.TrimSpace(event.Status)
	event.RemoteIP = strings.TrimSpace(event.RemoteIP)
	if event.ActorType == "" {
		event.ActorType = "system"
	}
	if event.Status == "" {
		event.Status = "unknown"
	}
	s.mu.Lock()
	now := time.Now().Unix()
	if event.CreatedAt == 0 {
		event.CreatedAt = now
	}
	if strings.TrimSpace(event.EventID) == "" {
		event.EventID = newAuditEventID(s.nextAuditSeq)
		s.nextAuditSeq++
	}
	event.Details = sanitizeAuditDetails(event.Details)
	s.auditEvents[event.EventID] = event
	s.mu.Unlock()
	if err := s.persistPostgresAuditEvent(context.Background(), event); err != nil {
		log.Printf("service-biz persist audit event failed action=%s actor=%s: %v", event.Action, event.ActorID, err)
	}
	return event
}

func newAuditEventID(seq int) string {
	if token, err := secureTokenHex(8); err == nil {
		return "audit-" + token
	}
	return fmt.Sprintf("audit-%06d", seq)
}

func (s *Store) ListAuditEvents() []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AuditEvent, 0, len(s.auditEvents))
	for _, event := range s.auditEvents {
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].EventID < out[j].EventID
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

func (s *Store) QueryAuditEvents(filter AuditEventFilter) ([]AuditEvent, error) {
	filter.normalize()
	if s.db != nil {
		return s.queryPostgresAuditEvents(context.Background(), filter)
	}
	events := s.ListAuditEvents()
	out := make([]AuditEvent, 0, len(events))
	for _, event := range events {
		if !filter.matches(event) {
			continue
		}
		out = append(out, event)
		if len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (filter *AuditEventFilter) normalize() {
	filter.ActorType = strings.TrimSpace(filter.ActorType)
	filter.ActorID = strings.TrimSpace(filter.ActorID)
	filter.Action = strings.TrimSpace(filter.Action)
	filter.ResourceType = strings.TrimSpace(filter.ResourceType)
	filter.ResourceID = strings.TrimSpace(filter.ResourceID)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Limit <= 0 || filter.Limit > 500 {
		filter.Limit = 100
	}
}

func (filter AuditEventFilter) matches(event AuditEvent) bool {
	return (filter.ActorType == "" || event.ActorType == filter.ActorType) &&
		(filter.ActorID == "" || event.ActorID == filter.ActorID) &&
		(filter.Action == "" || event.Action == filter.Action) &&
		(filter.ResourceType == "" || event.ResourceType == filter.ResourceType) &&
		(filter.ResourceID == "" || event.ResourceID == filter.ResourceID) &&
		(filter.Status == "" || event.Status == filter.Status)
}

func sanitizeAuditDetails(details map[string]string) map[string]string {
	if len(details) == 0 {
		return nil
	}
	out := make(map[string]string, len(details))
	for key, value := range details {
		normalized := strings.ToLower(strings.TrimSpace(key))
		switch {
		case normalized == "":
			continue
		case isSensitiveFieldName(normalized):
			out[normalized] = "[redacted]"
		default:
			out[normalized] = strings.TrimSpace(value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *Store) queryPostgresAuditEvents(ctx context.Context, filter AuditEventFilter) ([]AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `select id,actor_type,coalesce(actor_id,''),coalesce(actor_email,''),action,coalesce(resource_type,''),coalesce(resource_id,''),status,coalesce(remote_ip::text,''),details::text,extract(epoch from created_at)::bigint
		from audit_events
		where ($1='' or actor_type=$1)
			and ($2='' or actor_id=$2)
			and ($3='' or action=$3)
			and ($4='' or resource_type=$4)
			and ($5='' or resource_id=$5)
			and ($6='' or status=$6)
		order by created_at desc, id desc
		limit $7`,
		filter.ActorType, filter.ActorID, filter.Action, filter.ResourceType, filter.ResourceID, filter.Status, filter.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AuditEvent, 0)
	for rows.Next() {
		var event AuditEvent
		detailsJSON := "{}"
		if err := rows.Scan(&event.EventID, &event.ActorType, &event.ActorID, &event.ActorEmail, &event.Action, &event.ResourceType, &event.ResourceID, &event.Status, &event.RemoteIP, &detailsJSON, &event.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(detailsJSON), &event.Details); err != nil {
			return nil, err
		}
		event.Details = sanitizeAuditDetails(event.Details)
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Store) persistPostgresAuditEvent(ctx context.Context, event AuditEvent) error {
	if s.db == nil {
		return nil
	}
	details, err := json.Marshal(event.Details)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `insert into audit_events(id,actor_type,actor_id,actor_email,action,resource_type,resource_id,status,remote_ip,details,created_at)
		values($1,$2,$3,$4,$5,$6,$7,$8,nullif($9,'')::inet,$10,to_timestamp($11))
		on conflict(id) do nothing`,
		event.EventID, event.ActorType, event.ActorID, event.ActorEmail, event.Action, event.ResourceType, event.ResourceID, event.Status, event.RemoteIP, string(details), event.CreatedAt)
	return err
}
