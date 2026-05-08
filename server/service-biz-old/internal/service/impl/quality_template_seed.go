package impl

import (
	"context"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s *dbState) seedDefaultRelayPolicyTemplates(ctx context.Context) error {
	now := time.Now().Unix()
	templates := []struct {
		id  string
		req dto.RelayPolicyTemplateRequest
	}{
		{id: "relay-template-stable", req: dto.RelayPolicyTemplateRequest{Name: "稳定优先", RelayMtu: 1280, MaxFramePayload: 1200, RecommendationLevel: 2, ExecutionLevel: 1, PathType: "any", PathStrategy: "bandwidth_saving", TTLMinutes: 60, Reason: "template_stable_client_quality"}},
		{id: "relay-template-performance", req: dto.RelayPolicyTemplateRequest{Name: "性能优先", RelayMtu: 1400, MaxFramePayload: 1320, RecommendationLevel: 4, ExecutionLevel: 2, PathType: "any", PathStrategy: "performance", TTLMinutes: 60, Reason: "template_performance_client_quality"}},
		{id: "relay-template-relay-saving", req: dto.RelayPolicyTemplateRequest{Name: "Relay 省流", RelayMtu: 1200, MaxFramePayload: 1120, RecommendationLevel: 3, ExecutionLevel: 2, PathType: "relay_udp", PathStrategy: "relay_saving", TTLMinutes: 120, Reason: "template_relay_saving_client_quality"}},
		{id: "relay-template-derp-fallback", req: dto.RelayPolicyTemplateRequest{Name: "DERP 保底", RelayMtu: 1180, MaxFramePayload: 1080, RecommendationLevel: 5, ExecutionLevel: 3, PathType: "derp_tcp_tls_443", PathStrategy: "relay_saving", TTLMinutes: 120, Reason: "template_derp_fallback_client_quality"}},
	}
	for _, item := range templates {
		record, err := relayPolicyTemplateRecord(item.id, "", item.req, true, now, now)
		if err != nil {
			return err
		}
		if err := s.pg.UpsertRelayPolicyTemplate(ctx, record); err != nil {
			if repo.IsUniqueViolation(err) {
				continue
			}
			return err
		}
	}
	return nil
}
