package service

import (
	"context"
	"encoding/json"
	"fmt"
)

type NetworkEventDeliveryRetryResult struct {
	Scanned     int
	Republished int
	Deleted     int64
}

func (s MQTTWebhookService) RetryNetworkEventDeliveries(ctx context.Context) (NetworkEventDeliveryRetryResult, error) {
	var result NetworkEventDeliveryRetryResult
	if s.EventDeliveries == nil || s.EventPublisher == nil {
		return result, nil
	}
	now := currentTime(s.Now).Unix()
	deleted, err := s.EventDeliveries.DeleteExpiredNetworkEventDeliveries(ctx, now)
	if err != nil {
		return result, err
	}
	result.Deleted = deleted
	items, err := s.EventDeliveries.ListDueNetworkEventDeliveries(ctx, now, 500)
	if err != nil {
		return result, err
	}
	var firstErr error
	for _, item := range items {
		result.Scanned++
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
			item.NextRetryAt = now + int64(networkJoinDeliveryRetryInterval.Seconds())
			result.Republished++
		}
		item.UpdatedAt = now
		if err := s.EventDeliveries.SaveNetworkEventDelivery(ctx, item); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return result, firstErr
}
