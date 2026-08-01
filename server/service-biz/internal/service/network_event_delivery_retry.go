package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/slan/service-biz/internal/model"
)

type NetworkEventDeliveryRetryResult struct {
	Scanned           int
	Republished       int
	FallbackPublished int
	Cleaned           int64
}

const networkEventDeliveryTerminalRetention = 24 * time.Hour
const networkEventDeliveryFallbackRetryInterval = 5 * time.Minute
const networkEventDeliveryLeaseDuration = 3 * time.Minute

func (s MQTTWebhookService) RetryNetworkEventDeliveries(ctx context.Context) (NetworkEventDeliveryRetryResult, error) {
	var result NetworkEventDeliveryRetryResult
	if s.EventDeliveries == nil {
		return result, nil
	}
	now := currentTime(s.Now).Unix()
	cleaned, err := s.EventDeliveries.DeleteTerminalNetworkEventDeliveriesBefore(ctx, now-int64(networkEventDeliveryTerminalRetention/time.Second))
	if err != nil {
		return result, err
	}
	result.Cleaned = cleaned
	if s.EventPublisher == nil && (s.DevicePublisher == nil || s.Networks == nil) {
		return result, nil
	}
	items, err := s.EventDeliveries.ClaimDueNetworkEventDeliveries(
		ctx,
		now,
		now+int64(networkEventDeliveryLeaseDuration/time.Second),
		500,
	)
	if err != nil {
		return result, err
	}
	var firstErr error
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			break
		}
		result.Scanned++
		if now >= item.ExpiresAt {
			item.Attempts++
			if err := s.publishPendingNetworkDelivery(ctx, item, true); err != nil {
				item.LastError = boundedNetworkDeliveryError(err.Error())
				if firstErr == nil {
					firstErr = fmt.Errorf("publish delivery fallback: %w", err)
				}
				item.NextRetryAt = now + 60
			} else {
				item.LastError = ""
				item.NextRetryAt = now + int64(networkEventDeliveryFallbackRetryInterval/time.Second)
				result.FallbackPublished++
			}
		} else {
			var event NetworkEventEnvelope
			if err := json.Unmarshal([]byte(item.Payload), &event); err != nil {
				item.Status = "invalid"
				item.LastError = boundedNetworkDeliveryError(err.Error())
				if firstErr == nil {
					firstErr = fmt.Errorf("decode pending network event: %w", err)
				}
			} else {
				item.Attempts++
				if err := s.publishPendingNetworkDelivery(ctx, item, false); err != nil {
					item.LastError = boundedNetworkDeliveryError(err.Error())
					if firstErr == nil {
						firstErr = fmt.Errorf("republish pending network event: %w", err)
					}
					item.NextRetryAt = now + 60
				} else {
					item.LastError = ""
					item.NextRetryAt = now + 60
					result.Republished++
				}
			}
		}
		item.UpdatedAt = now
		if _, err := s.EventDeliveries.UpdatePendingNetworkEventDelivery(ctx, item); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return result, firstErr
}

func (s MQTTWebhookService) publishPendingNetworkDelivery(ctx context.Context, item model.NetworkEventDelivery, fallback bool) error {
	if s.DevicePublisher != nil && s.Networks != nil {
		operation := "joined"
		if item.EventType == string(NetworkEventMemberRemoved) {
			operation = "left"
		}
		return publishDirectNetworkMembershipWithMessageID(
			ctx,
			s.Networks,
			s.DevicePublisher,
			item.TargetDeviceID,
			item.NetworkID,
			operation,
			item.ConfigVersion,
			item.EventID,
		)
	}
	if fallback {
		event := newNetworkEventEnvelope(
			NetworkEventVersion,
			item.NetworkID,
			uint64(item.ConfigVersion),
			currentTime(s.Now).UnixMilli(),
			map[string]any{"configVersion": item.ConfigVersion},
		)
		return s.EventPublisher.PublishNetworkEvent(ctx, event)
	}
	var event NetworkEventEnvelope
	if err := json.Unmarshal([]byte(item.Payload), &event); err != nil {
		return fmt.Errorf("decode pending network event: %w", err)
	}
	return s.EventPublisher.PublishNetworkEvent(ctx, event)
}
