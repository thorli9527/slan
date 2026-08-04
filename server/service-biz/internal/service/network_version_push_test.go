package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestPushNetworkVersionHeartbeatsOnlyPublishesNetworksWithTwoActiveDevices(t *testing.T) {
	now := time.Unix(1_700_010_000, 0)
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-one": {NetworkID: "net-one"},
			"net-two": {NetworkID: "net-two"},
		},
		versions: map[string]model.NetworkConfigVersion{
			"net-one": {NetworkID: "net-one", Version: 4},
			"net-two": {NetworkID: "net-two", Version: 9},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-one": {
				{NetworkID: "net-one", DeviceID: "device-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
			"net-two": {
				{NetworkID: "net-two", DeviceID: "device-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
				{NetworkID: "net-two", DeviceID: "device-2", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
				{NetworkID: "net-two", DeviceID: "device-2", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
				{NetworkID: "net-two", DeviceID: "device-disabled", Enabled: false, MemberStatus: model.NetworkMemberStatusDisabled},
			},
		},
	}
	publisher := &deviceRuntimeTestEventPublisher{}
	service := NetworkCoreService{
		Networks: networks, EventPublisher: publisher,
		VersionPushTracker: NewNetworkVersionPushTracker(), Now: func() time.Time { return now },
	}

	result, err := service.PushNetworkVersionHeartbeats(context.Background())
	if err != nil {
		t.Fatalf("PushNetworkVersionHeartbeats returned error: %v", err)
	}
	if result.ScannedNetworks != 2 || result.EligibleNetworks != 1 || result.PublishedNetworks != 1 {
		t.Fatalf("unexpected push result: %+v", result)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected one version event, got %d", len(publisher.events))
	}
	event := publisher.events[0]
	if event.NetworkID != "net-two" || event.Version != 9 || event.EventType != NetworkEventVersion {
		t.Fatalf("unexpected version event: %+v", event)
	}

	second, err := service.PushNetworkVersionHeartbeats(context.Background())
	if err != nil {
		t.Fatalf("second PushNetworkVersionHeartbeats returned error: %v", err)
	}
	if second.PublishedNetworks != 0 || second.SkippedUnchanged != 1 || len(publisher.events) != 1 {
		t.Fatalf("unchanged version should be skipped: result=%+v events=%d", second, len(publisher.events))
	}

	version := networks.versions["net-two"]
	version.Version = 10
	networks.versions["net-two"] = version
	third, err := service.PushNetworkVersionHeartbeats(context.Background())
	if err != nil {
		t.Fatalf("third PushNetworkVersionHeartbeats returned error: %v", err)
	}
	if third.PublishedNetworks != 1 || len(publisher.events) != 2 || publisher.events[1].Version != 10 {
		t.Fatalf("new version should be published: result=%+v events=%+v", third, publisher.events)
	}
}
