package impl

import (
	"context"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbOpsService) WireNodes() (dto.WireNodesOpsView, error) {
	return dbWireService{state: s.state}.WireNodesOps()
}

func (s dbOpsService) WireNodeEvents(query dto.WireNodeEventQuery) (dto.WireNodeEventListResponse, error) {
	query = normalizeWireNodeEventQuery(query)
	events, total, err := s.state.pg.QueryWireNodeEvents(context.Background(), repo.WireNodeEventQuery{
		NodeKind:      query.NodeKind,
		RegionID:      query.RegionID,
		NodeID:        query.NodeID,
		EventType:     query.EventType,
		CreatedFromMs: query.CreatedFromMs,
		CreatedToMs:   query.CreatedToMs,
		Limit:         query.PageSize,
		Offset:        (query.Page - 1) * query.PageSize,
	})
	if err != nil {
		return dto.WireNodeEventListResponse{}, err
	}
	out := make([]dto.WireNodeEventRecord, 0, len(events))
	for _, event := range events {
		out = append(out, event.ToDTO())
	}
	return dto.WireNodeEventListResponse{
		Items:    out,
		Page:     query.Page,
		PageSize: query.PageSize,
		Total:    total,
	}, nil
}

func normalizeWireNodeEventQuery(query dto.WireNodeEventQuery) dto.WireNodeEventQuery {
	query.NodeKind = strings.TrimSpace(strings.ToLower(query.NodeKind))
	query.RegionID = strings.TrimSpace(query.RegionID)
	query.NodeID = strings.TrimSpace(query.NodeID)
	query.EventType = strings.TrimSpace(strings.ToLower(query.EventType))
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 50
	}
	if query.PageSize > 200 {
		query.PageSize = 200
	}
	return query
}
