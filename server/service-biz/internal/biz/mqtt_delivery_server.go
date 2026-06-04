package biz

import (
	"context"
	"log"
	"strings"
	"time"
)

func (s *Server) notifyDeviceUserLoginSucceeded(deviceID string, payload DeviceUserLoginPayload) (string, error) {
	if !s.mqtt.Enabled || strings.TrimSpace(deviceID) == "" {
		return "", errUnavailable
	}
	deliveryID := mqttMessageID("msg")
	err := s.publishDeviceControlWithDelivery(deviceID, "device_user_login_succeeded", "login", deliveryID, payload)
	if err == nil {
		log.Printf("mqtt publish device_user_login_succeeded succeeded device=%s user=%s deliveryId=%s", deviceID, payload.UserID, deliveryID)
		return deliveryID, nil
	}
	if _, queuedErr := s.services.MQTT.DeliveryForDevice(deviceID, deliveryID); queuedErr != nil {
		return "", err
	}
	log.Printf("mqtt publish device_user_login_succeeded queued for retry device=%s user=%s deliveryId=%s err=%v", deviceID, payload.UserID, deliveryID, err)
	return deliveryID, nil
}

func (s *Server) publishNetworkConfigChanged(deviceIDs []string, payload networkChangePayload) {
	for _, deviceID := range deviceIDs {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
		deliveryID := mqttMessageID("msg")
		devicePayload := s.networkChangePayloadForDevice(payload, deviceID)
		err := s.publishDeviceControlWithDeliveryContext(ctx, deviceID, "network_config_changed", payload.Action, deliveryID, devicePayload)
		cancel()
		if err != nil {
			log.Printf("mqtt publish network_config_changed failed device=%s network=%s: %v", deviceID, payload.NetworkID, err)
		}
	}
}

func (s *Server) networkChangePayloadForDevice(payload networkChangePayload, deviceID string) networkChangePayload {
	s.ensureServices()
	config, err := s.services.MQTT.NetworkConfig(payload.NetworkID, deviceID)
	if err != nil {
		return payload
	}
	payload.VirtualIP = config.GlobalIP
	payload.PrefixLen = config.PrefixLen
	payload.GlobalCIDR = config.GlobalCIDR
	payload.SubnetID = config.SubnetID
	payload.SubnetCIDR = config.SubnetCIDR
	payload.SubnetPrefixLen = config.SubnetPrefixLen
	return payload
}

func (s *Server) publishDeviceControlWithDelivery(deviceID, messageType, action, deliveryID string, payload any) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
	defer cancel()
	return s.publishDeviceControlWithDeliveryContext(ctx, deviceID, messageType, action, deliveryID, payload)
}

func (s *Server) publishDeviceControlWithDeliveryContext(ctx context.Context, deviceID, messageType, action, deliveryID string, payload any) error {
	now := timeNow().Unix()
	expiresAt := now + int64(s.mqtt.ControlMessageTTLSeconds)
	if _, err := s.services.MQTT.PrepareDelivery(deviceID, deliveryID, messageType, action, payload, now, expiresAt); err != nil {
		return err
	}
	err := publishControlMQTTWithMessageID(ctx, s.mqtt, deviceID, messageType, deliveryID, payload)
	resultAt := timeNow().Unix()
	if err != nil {
		_, recordErr := s.services.MQTT.RecordPublishResult(deviceID, deliveryID, false, err.Error(), resultAt)
		if recordErr != nil {
			log.Printf("mqtt record publish failure failed device=%s deliveryId=%s: %v", deviceID, deliveryID, recordErr)
		}
		return err
	}
	if _, recordErr := s.services.MQTT.RecordPublishResult(deviceID, deliveryID, true, "", resultAt); recordErr != nil {
		return recordErr
	}
	return nil
}
