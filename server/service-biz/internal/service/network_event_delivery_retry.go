package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type NetworkEventDeliveryRetryResult struct {
	Scanned           int
	Republished       int
	FallbackPublished int
}

func (s MQTTWebhookService) RetryNetworkEventDeliveries(ctx context.Context) (NetworkEventDeliveryRetryResult, error) {
	var result NetworkEventDeliveryRetryResult
	if s.EventDeliveries == nil || s.EventPublisher == nil {
		return result, nil
	}
	now := currentTime(s.Now).Unix()
	items, err := s.EventDeliveries.ListDueNetworkEventDeliveries(ctx, now, 500)
	if err != nil {
		return result, err
	}
	var firstErr error
	for _, item := range items {
		result.Scanned++
		if now >= item.ExpiresAt {
			event := newNetworkEventEnvelope(NetworkEventVersion, item.NetworkID, uint64(item.ConfigVersion), time.Unix(now, 0).UnixMilli(), map[string]any{"configVersion": item.ConfigVersion})
			if err := s.EventPublisher.PublishNetworkEvent(ctx, event); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("publish delivery fallback: %w", err)
				}
				item.NextRetryAt = now + 60
			} else {
				item.Status = "fallback"
				result.FallbackPublished++
			}
		} else {
			var event NetworkEventEnvelope
			if err := json.Unmarshal([]byte(item.Payload), &event); err != nil {
				item.Status = "invalid"
				if firstErr == nil {
					firstErr = fmt.Errorf("decode pending network event: %w", err)
				}
			} else if err := s.EventPublisher.PublishNetworkEvent(ctx, event); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("republish pending network event: %w", err)
				}
			} else {
				item.Attempts++
				item.NextRetryAt = now + 60
				result.Republished++
			}
		}
		item.UpdatedAt = now
		if err := s.EventDeliveries.SaveNetworkEventDelivery(ctx, item); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return result, firstErr
}
