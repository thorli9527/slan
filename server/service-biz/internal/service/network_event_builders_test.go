package service

import (
	"testing"

	"github.com/slan/service-biz/internal/model"
)

func TestNetworkEventACLRulesMapsDeviceAndGroupSources(t *testing.T) {
	rules := networkEventACLRules([]model.SecurityRule{
		{
			RuleID:    "rule-device",
			PeerType:  "device",
			PeerValue: "device-1",
			PortRange: "19090",
		},
		{
			RuleID:    "rule-group",
			PeerType:  "device_group",
			PeerValue: "group-1",
			PortRange: "443",
		},
	})

	if got := rules[0].SourceDeviceIDs; len(got) != 1 || got[0] != "device-1" {
		t.Fatalf("expected device source id mapping, got %+v", got)
	}
	if got := rules[0].SourceGroupIDs; len(got) != 0 {
		t.Fatalf("expected no group source ids for device rule, got %+v", got)
	}
	if got := rules[1].SourceGroupIDs; len(got) != 1 || got[0] != "group-1" {
		t.Fatalf("expected group source id mapping, got %+v", got)
	}
	if got := rules[1].SourceDeviceIDs; len(got) != 0 {
		t.Fatalf("expected no device source ids for group rule, got %+v", got)
	}
}

func TestBuildNetworkEventSnapshotPayloadMapsAclSources(t *testing.T) {
	payload := buildNetworkEventSnapshotPayload(NetworkResolvedConfigView{
		Config: NetworkConfigView{
			Network: NetworkView{
				NetworkID: "net-1",
				Name:      "Default",
			},
			DeviceID:   "device-local",
			GlobalIP:   "10.0.1.10",
			GlobalName: "local",
			SecurityRules: []SecurityRuleView{
				{
					RuleID:    "rule-device",
					PeerType:  "device",
					PeerValue: "device-peer",
					PortRange: "19090",
				},
				{
					RuleID:    "rule-group",
					PeerType:  "device_group",
					PeerValue: "group-1",
					PortRange: "443",
				},
			},
		},
	})

	if got := payload.ACLRules[0].SourceDeviceIDs; len(got) != 1 || got[0] != "device-peer" {
		t.Fatalf("expected snapshot device source id mapping, got %+v", got)
	}
	if got := payload.ACLRules[1].SourceGroupIDs; len(got) != 1 || got[0] != "group-1" {
		t.Fatalf("expected snapshot group source id mapping, got %+v", got)
	}
}
