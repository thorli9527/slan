package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

type deliveryTestStore struct {
	mu      sync.Mutex
	items   map[string]model.NetworkEventDelivery
	members []model.NetworkDevice
}

func (s *deliveryTestStore) key(eventID, deviceID string) string { return eventID + ":" + deviceID }
func (s *deliveryTestStore) GetNetworkEventDelivery(_ context.Context, eventID, deviceID string) (model.NetworkEventDelivery, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[s.key(eventID, deviceID)]
	return item, ok, nil
}
func (s *deliveryTestStore) ListDueNetworkEventDeliveries(_ context.Context, dueAt int64, _ int) ([]model.NetworkEventDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]model.NetworkEventDelivery, 0)
	for _, item := range s.items {
		if item.Status == "pending" && item.NextRetryAt <= dueAt {
			items = append(items, item)
		}
	}
	return items, nil
}
func (s *deliveryTestStore) SaveNetworkEventDelivery(_ context.Context, item model.NetworkEventDelivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = map[string]model.NetworkEventDelivery{}
	}
	s.items[s.key(item.EventID, item.TargetDeviceID)] = item
	return nil
}
func (s *deliveryTestStore) ListNetworkDevices(_ context.Context, _ string) ([]model.NetworkDevice, error) {
	return append([]model.NetworkDevice(nil), s.members...), nil
}

type deliveryTestPublisher struct{ events []NetworkEventEnvelope }

func (p *deliveryTestPublisher) PublishNetworkEvent(_ context.Context, event NetworkEventEnvelope) error {
	p.events = append(p.events, event)
	return nil
}

func TestMembershipEventCreatesPerDeviceDeliveryRecords(t *testing.T) {
	store := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}, members: []model.NetworkDevice{
		{DeviceID: "device-a", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
		{DeviceID: "device-b", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
	}}
	publisher := &MqttNetworkEventPublisher{deliveries: store}
	event := newNetworkEventEnvelope(NetworkEventMemberRemoved, "network-1", 7, time.Unix(100, 0).UnixMilli(), NetworkEventMemberRemovedPayload{DeviceID: "device-c"})
	payload, _ := json.Marshal(event)
	if err := publisher.trackMembershipEvent(context.Background(), event, payload); err != nil {
		t.Fatal(err)
	}
	if len(store.items) != 3 {
		t.Fatalf("expected delivery rows for two members and removed device, got %d", len(store.items))
	}
	for _, item := range store.items {
		if item.Status != "pending" || item.ExpiresAt-item.CreatedAt != 300 {
			t.Fatalf("unexpected delivery: %+v", item)
		}
	}
}

func TestNetworkEventDeliveryAckAndRetryFallback(t *testing.T) {
	now := time.Unix(1_000, 0)
	event := newNetworkEventEnvelope(NetworkEventMemberAdded, "network-1", 9, now.Add(-time.Minute).UnixMilli(), NetworkEventMemberPayload{})
	payload, _ := json.Marshal(event)
	store := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}}
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{EventID: event.EventID, TargetDeviceID: "device-a", NetworkID: "network-1", EventType: string(event.EventType), ConfigVersion: 9, Payload: string(payload), Status: "pending", Attempts: 1, NextRetryAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix()})
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{EventID: "expired", TargetDeviceID: "device-b", NetworkID: "network-1", EventType: string(NetworkEventMemberRemoved), ConfigVersion: 9, Payload: string(payload), Status: "pending", Attempts: 5, NextRetryAt: now.Unix(), ExpiresAt: now.Add(-time.Second).Unix()})
	publisher := &deliveryTestPublisher{}
	service := MQTTWebhookService{EventDeliveries: store, EventPublisher: publisher, Now: func() time.Time { return now }}
	result, err := service.RetryNetworkEventDeliveries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Republished != 1 || result.FallbackPublished != 1 {
		t.Fatalf("unexpected retry result: %+v", result)
	}
	ack := []byte(`{"deliveryId":"` + event.EventID + `","status":"succeeded"}`)
	if err := service.handleNetworkEventDeliveryAck(context.Background(), "slan/devices/device-a/control/ack", ack); err != nil {
		t.Fatal(err)
	}
	item, _, _ := store.GetNetworkEventDelivery(context.Background(), event.EventID, "device-a")
	if item.Status != "acknowledged" || item.AcknowledgedAt != now.Unix() {
		t.Fatalf("unexpected acknowledged delivery: %+v", item)
	}
}
