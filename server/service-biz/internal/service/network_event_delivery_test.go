package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

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

func (s *deliveryTestStore) ClaimDueNetworkEventDeliveries(_ context.Context, dueAt, leaseUntil int64, limit int) ([]model.NetworkEventDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]model.NetworkEventDelivery, 0)
	for key, item := range s.items {
		if item.Status != "pending" || item.NextRetryAt > dueAt {
			continue
		}
		item.NextRetryAt = leaseUntil
		s.items[key] = item
		items = append(items, item)
		if limit > 0 && len(items) >= limit {
			break
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

func (s *deliveryTestStore) UpdatePendingNetworkEventDelivery(_ context.Context, item model.NetworkEventDelivery) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.key(item.EventID, item.TargetDeviceID)
	current, ok := s.items[key]
	if !ok || current.Status != "pending" {
		return false, nil
	}
	s.items[key] = item
	return true, nil
}

func (s *deliveryTestStore) DeleteTerminalNetworkEventDeliveriesBefore(_ context.Context, cutoff int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted int64
	for key, item := range s.items {
		if item.Status != "pending" && item.UpdatedAt < cutoff {
			delete(s.items, key)
			deleted++
		}
	}
	return deleted, nil
}

func TestRetryCleansOnlyExpiredTerminalDeliveries(t *testing.T) {
	now := time.Unix(200_000, 0)
	old := now.Add(-networkEventDeliveryTerminalRetention - time.Minute).Unix()
	recent := now.Add(-time.Hour).Unix()
	store := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}}
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{
		EventID: "old-terminal", TargetDeviceID: "device-a", Status: "acknowledged", UpdatedAt: old,
	})
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{
		EventID: "recent-terminal", TargetDeviceID: "device-b", Status: "fallback", UpdatedAt: recent,
	})
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{
		EventID: "old-pending", TargetDeviceID: "device-c", Status: "pending", UpdatedAt: old,
		NextRetryAt: now.Add(time.Hour).Unix(),
	})
	service := MQTTWebhookService{
		EventDeliveries: store,
		EventPublisher:  &deliveryTestPublisher{},
		Now:             func() time.Time { return now },
	}

	result, err := service.RetryNetworkEventDeliveries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Cleaned != 1 {
		t.Fatalf("expected one cleaned terminal delivery, got %+v", result)
	}
	if _, ok, _ := store.GetNetworkEventDelivery(context.Background(), "old-terminal", "device-a"); ok {
		t.Fatal("old terminal delivery was not deleted")
	}
	if _, ok, _ := store.GetNetworkEventDelivery(context.Background(), "recent-terminal", "device-b"); !ok {
		t.Fatal("recent terminal delivery was deleted")
	}
	if _, ok, _ := store.GetNetworkEventDelivery(context.Background(), "old-pending", "device-c"); !ok {
		t.Fatal("pending delivery was deleted")
	}
}

func TestNetworkEventDeliveryClaimPreventsDuplicateWorkers(t *testing.T) {
	store := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}}
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{
		EventID: "claim-event", TargetDeviceID: "device-a", Status: "pending", NextRetryAt: 100,
	})
	first, err := store.ClaimDueNetworkEventDeliveries(context.Background(), 100, 280, 10)
	if err != nil || len(first) != 1 || first[0].NextRetryAt != 280 {
		t.Fatalf("unexpected first claim: items=%+v err=%v", first, err)
	}
	second, err := store.ClaimDueNetworkEventDeliveries(context.Background(), 100, 280, 10)
	if err != nil || len(second) != 0 {
		t.Fatalf("delivery was claimed twice during lease: items=%+v err=%v", second, err)
	}
	recovered, err := store.ClaimDueNetworkEventDeliveries(context.Background(), 280, 460, 10)
	if err != nil || len(recovered) != 1 {
		t.Fatalf("expired lease was not recoverable: items=%+v err=%v", recovered, err)
	}
}

func TestRetryCleansTerminalDeliveriesWhenPublisherIsDisabled(t *testing.T) {
	now := time.Unix(300_000, 0)
	store := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}}
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{
		EventID: "disabled-publisher-terminal", TargetDeviceID: "device-a", Status: "invalid",
		UpdatedAt: now.Add(-networkEventDeliveryTerminalRetention - time.Second).Unix(),
	})
	service := MQTTWebhookService{EventDeliveries: store, Now: func() time.Time { return now }}

	result, err := service.RetryNetworkEventDeliveries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Cleaned != 1 || result.Scanned != 0 {
		t.Fatalf("unexpected cleanup-only result: %+v", result)
	}
}

func TestNetworkEventRetryStopsBeforePublishingWhenContextIsCanceled(t *testing.T) {
	now := time.Unix(350_000, 0)
	store := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}}
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{
		EventID: "canceled-event", TargetDeviceID: "device-a", NetworkID: "network-1",
		EventType: string(NetworkEventMemberAdded), Payload: `{}`, Status: "pending",
		NextRetryAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(),
	})
	publisher := &deliveryTestPublisher{}
	service := MQTTWebhookService{
		EventDeliveries: store,
		EventPublisher:  publisher,
		Now:             func() time.Time { return now },
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := service.RetryNetworkEventDeliveries(ctx)
	if err != context.Canceled {
		t.Fatalf("expected context cancellation, got result=%+v err=%v", result, err)
	}
	if result.Scanned != 0 || len(publisher.events) != 0 {
		t.Fatalf("canceled retry published work: result=%+v events=%d", result, len(publisher.events))
	}
}
func (s *deliveryTestStore) ListNetworkDevices(_ context.Context, _ string) ([]model.NetworkDevice, error) {
	return append([]model.NetworkDevice(nil), s.members...), nil
}

type deliveryTestPublisher struct {
	events []NetworkEventEnvelope
	err    error
}

func (p *deliveryTestPublisher) PublishNetworkEvent(_ context.Context, event NetworkEventEnvelope) error {
	p.events = append(p.events, event)
	return p.err
}

func TestNetworkEventRetryRecordsServerPublishFailure(t *testing.T) {
	now := time.Unix(800, 0)
	event := newNetworkEventEnvelope(NetworkEventMemberAdded, "network-1", 3, now.UnixMilli(), NetworkEventMemberPayload{})
	payload, _ := json.Marshal(event)
	store := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}}
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{
		EventID: event.EventID, TargetDeviceID: "device-a", NetworkID: "network-1",
		EventType: string(event.EventType), ConfigVersion: 3, Payload: string(payload),
		Status: "pending", Attempts: 1, NextRetryAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(),
	})
	service := MQTTWebhookService{
		EventDeliveries: store,
		EventPublisher:  &deliveryTestPublisher{err: errors.New("broker unavailable")},
		Now:             func() time.Time { return now },
	}

	result, err := service.RetryNetworkEventDeliveries(context.Background())
	if err == nil || result.Scanned != 1 || result.Republished != 0 {
		t.Fatalf("unexpected failed retry result: result=%+v err=%v", result, err)
	}
	stored, ok, loadErr := store.GetNetworkEventDelivery(context.Background(), event.EventID, "device-a")
	if loadErr != nil || !ok {
		t.Fatalf("load failed delivery: ok=%v err=%v", ok, loadErr)
	}
	if stored.Status != "pending" || stored.Attempts != 2 || stored.NextRetryAt != now.Add(time.Minute).Unix() || stored.LastError != "broker unavailable" {
		t.Fatalf("server publish failure was not persisted: %+v", stored)
	}
}

type deliveryTestDevicePublisher struct {
	deviceIDs []string
	events    []DeviceControlEnvelope
	onPublish func()
}

func (p *deliveryTestDevicePublisher) PublishDeviceControl(_ context.Context, deviceID string, event DeviceControlEnvelope) error {
	p.deviceIDs = append(p.deviceIDs, deviceID)
	p.events = append(p.events, event)
	if p.onPublish != nil {
		p.onPublish()
	}
	return nil
}

func TestNetworkEventRetryTargetsRemovedDeviceDirectly(t *testing.T) {
	now := time.Unix(1_500, 0)
	event := newNetworkEventEnvelope(NetworkEventMemberRemoved, "network-1", 11, now.Add(-time.Minute).UnixMilli(), NetworkEventMemberRemovedPayload{DeviceID: "device-a"})
	payload, _ := json.Marshal(event)
	store := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}}
	store.SaveNetworkEventDelivery(context.Background(), model.NetworkEventDelivery{
		EventID: event.EventID, TargetDeviceID: "device-a", NetworkID: "network-1",
		EventType: string(event.EventType), ConfigVersion: 11, Payload: string(payload),
		Status: "pending", Attempts: 1, NextRetryAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(),
	})
	networks := &networkRuntimeTestNetworks{
		networks: map[string]model.Network{
			"network-2": {NetworkID: "network-2"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"network-2": {{NetworkID: "network-2", DeviceID: "device-a", Enabled: true}},
		},
	}
	devicePublisher := &deliveryTestDevicePublisher{}
	broadcastPublisher := &deliveryTestPublisher{}
	service := MQTTWebhookService{
		Networks: networks, EventDeliveries: store, EventPublisher: broadcastPublisher,
		DevicePublisher: devicePublisher, Now: func() time.Time { return now },
	}

	result, err := service.RetryNetworkEventDeliveries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Republished != 1 || len(devicePublisher.events) != 1 {
		t.Fatalf("expected one direct retry, result=%+v events=%d", result, len(devicePublisher.events))
	}
	if len(broadcastPublisher.events) != 0 {
		t.Fatalf("direct retry unexpectedly used network broadcast: %d", len(broadcastPublisher.events))
	}
	direct := devicePublisher.events[0]
	if devicePublisher.deviceIDs[0] != "device-a" || direct.MessageID != event.EventID {
		t.Fatalf("unexpected direct delivery identity: device=%q event=%+v", devicePublisher.deviceIDs[0], direct)
	}
	if direct.Payload["operation"] != "left" || direct.Payload["changedNetworkId"] != "network-1" {
		t.Fatalf("unexpected direct membership payload: %+v", direct.Payload)
	}
	networkIDs, ok := direct.Payload["networkIds"].([]string)
	if !ok || len(networkIDs) != 1 || networkIDs[0] != "network-2" {
		t.Fatalf("direct retry must include current full membership: %#v", direct.Payload["networkIds"])
	}
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

func TestNetworkEventRetryDoesNotOverwriteConcurrentAck(t *testing.T) {
	now := time.Unix(1_700, 0)
	event := newNetworkEventEnvelope(NetworkEventMemberRemoved, "network-1", 12, now.Add(-time.Minute).UnixMilli(), NetworkEventMemberRemovedPayload{DeviceID: "device-a"})
	payload, _ := json.Marshal(event)
	store := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}}
	pending := model.NetworkEventDelivery{
		EventID: event.EventID, TargetDeviceID: "device-a", NetworkID: "network-1",
		EventType: string(event.EventType), ConfigVersion: 12, Payload: string(payload),
		Status: "pending", Attempts: 1, NextRetryAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(),
	}
	store.SaveNetworkEventDelivery(context.Background(), pending)
	devicePublisher := &deliveryTestDevicePublisher{onPublish: func() {
		acknowledged := pending
		acknowledged.Status = "acknowledged"
		acknowledged.AcknowledgedAt = now.Unix()
		acknowledged.UpdatedAt = now.Unix()
		if err := store.SaveNetworkEventDelivery(context.Background(), acknowledged); err != nil {
			t.Errorf("save concurrent ack: %v", err)
		}
	}}
	service := MQTTWebhookService{
		Networks: &networkRuntimeTestNetworks{
			networks:       map[string]model.Network{},
			networkDevices: map[string][]model.NetworkDevice{},
		},
		EventDeliveries: store,
		DevicePublisher: devicePublisher,
		Now:             func() time.Time { return now },
	}

	if _, err := service.RetryNetworkEventDeliveries(context.Background()); err != nil {
		t.Fatal(err)
	}
	stored, ok, err := store.GetNetworkEventDelivery(context.Background(), event.EventID, "device-a")
	if err != nil || !ok {
		t.Fatalf("load delivery after retry: ok=%v err=%v", ok, err)
	}
	if stored.Status != "acknowledged" || stored.AcknowledgedAt != now.Unix() {
		t.Fatalf("retry overwrote concurrent acknowledgement: %+v", stored)
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
	expired, ok, err := store.GetNetworkEventDelivery(context.Background(), "expired", "device-b")
	if err != nil || !ok {
		t.Fatalf("load expired delivery: ok=%v err=%v", ok, err)
	}
	if expired.Status != "pending" || expired.Attempts != 6 || expired.NextRetryAt != now.Add(networkEventDeliveryFallbackRetryInterval).Unix() {
		t.Fatalf("fallback delivery must remain pending until client ack: %+v", expired)
	}
	foundVersionFallback := false
	for _, published := range publisher.events {
		if published.EventType == NetworkEventVersion {
			foundVersionFallback = true
			break
		}
	}
	if !foundVersionFallback {
		t.Fatalf("expired compatibility delivery did not publish a version refresh: %+v", publisher.events)
	}
	failedAck := []byte(`{"deliveryId":"` + event.EventID + `","status":"failed","error":"reconcile failed"}`)
	if err := service.handleNetworkEventDeliveryAck(context.Background(), "slan/devices/device-a/control/ack", failedAck); err != nil {
		t.Fatal(err)
	}
	item, _, _ := store.GetNetworkEventDelivery(context.Background(), event.EventID, "device-a")
	if item.Status != "pending" || item.NextRetryAt != now.Add(10*time.Second).Unix() || item.LastError != "reconcile failed" {
		t.Fatalf("failed ack did not schedule an early retry: %+v", item)
	}
	ack := []byte(`{"deliveryId":"` + event.EventID + `","status":"succeeded"}`)
	if err := service.handleNetworkEventDeliveryAck(context.Background(), "slan/devices/device-a/control/ack", ack); err != nil {
		t.Fatal(err)
	}
	item, _, _ = store.GetNetworkEventDelivery(context.Background(), event.EventID, "device-a")
	if item.Status != "acknowledged" || item.AcknowledgedAt != now.Unix() || item.LastError != "" {
		t.Fatalf("unexpected acknowledged delivery: %+v", item)
	}
}

func TestBoundedNetworkDeliveryErrorUsesUnicodeCharacters(t *testing.T) {
	value := strings.Repeat("错", 2050)
	bounded := boundedNetworkDeliveryError("  " + value + "  ")
	if got := len([]rune(bounded)); got != 2048 {
		t.Fatalf("unexpected bounded error length: %d", got)
	}
	if !utf8.ValidString(bounded) {
		t.Fatal("bounded error is not valid UTF-8")
	}
}
