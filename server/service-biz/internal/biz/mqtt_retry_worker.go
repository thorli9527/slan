package biz

import (
	"context"
	"log"
	"time"
)

func (s *Server) startMQTTDeliveryRetryWorker() {
	if !s.mqtt.Enabled {
		return
	}
	go s.runMQTTDeliveryRetryWorker(context.Background())
}

func (s *Server) runMQTTDeliveryRetryWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.retryMQTTControlDeliveries(ctx)
		}
	}
}

func (s *Server) retryMQTTControlDeliveries(ctx context.Context) {
	now := timeNow().Unix()
	if expired := s.services.MQTT.ExpireDeliveries(now); expired > 0 {
		log.Printf("mqtt control delivery expired count=%d", expired)
	}
	deliveries := s.services.MQTT.RetryableDeliveries(now, 100)
	for _, delivery := range deliveries {
		claimed, err := s.services.MQTT.ClaimRetry(delivery.DeviceID, delivery.DeliveryID, delivery.UpdatedAt, now)
		if err != nil {
			log.Printf("mqtt control delivery retry claim failed device=%s deliveryId=%s: %v", delivery.DeviceID, delivery.DeliveryID, err)
			continue
		}
		if !claimed {
			continue
		}
		if len(delivery.Payload) == 0 {
			log.Printf("mqtt control delivery retry skipped empty payload device=%s deliveryId=%s", delivery.DeviceID, delivery.DeliveryID)
			continue
		}
		publishCtx, cancel := context.WithTimeout(ctx, time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
		err = publishControlMQTTWithMessageID(publishCtx, s.mqtt, delivery.DeviceID, delivery.MessageType, delivery.DeliveryID, delivery.Payload)
		cancel()
		resultAt := timeNow().Unix()
		if err != nil {
			_, recordErr := s.services.MQTT.RecordPublishResult(delivery.DeviceID, delivery.DeliveryID, false, err.Error(), resultAt)
			if recordErr != nil {
				log.Printf("mqtt control delivery retry record failure failed device=%s deliveryId=%s: %v", delivery.DeviceID, delivery.DeliveryID, recordErr)
			}
			log.Printf("mqtt control delivery retry failed device=%s deliveryId=%s attempt=%d: %v", delivery.DeviceID, delivery.DeliveryID, delivery.AttemptCount+1, err)
			continue
		}
		if _, err := s.services.MQTT.RecordPublishResult(delivery.DeviceID, delivery.DeliveryID, true, "", resultAt); err != nil {
			log.Printf("mqtt control delivery retry record success failed device=%s deliveryId=%s: %v", delivery.DeviceID, delivery.DeliveryID, err)
			continue
		}
		log.Printf("mqtt control delivery retry published device=%s deliveryId=%s attempt=%d", delivery.DeviceID, delivery.DeliveryID, delivery.AttemptCount+1)
	}
}
