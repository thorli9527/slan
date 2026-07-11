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
		{
			RuleID:    "rule-cidr",
			PeerType:  "cidr",
			PeerValue: "10.0.0.0/24",
			PortRange: "53",
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
	if got := rules[2].SourceValues; len(got) != 1 || got[0] != "10.0.0.0/24" {
		t.Fatalf("expected generic source values mapping, got %+v", got)
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
				{
					RuleID:    "rule-cidr",
					PeerType:  "cidr",
					PeerValue: "10.0.0.0/24",
					PortRange: "53",
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
	if got := payload.ACLRules[2].SourceValues; len(got) != 1 || got[0] != "10.0.0.0/24" {
		t.Fatalf("expected snapshot generic source values mapping, got %+v", got)
	}
}

func TestNetworkEventDNSRecordsBuildsFullFQDN(t *testing.T) {
	records := networkEventDNSRecords(
		[]model.DNSRecord{
			{
				RecordID:  "rec-1",
				NetworkID: "net-1",
				ZoneID:    "zone-1",
				Name:      "self",
				Type:      "A",
				Value:     "device-1",
				TTL:       60,
			},
		},
		[]model.DNSZone{
			{
				ZoneID:    "zone-1",
				NetworkID: "net-1",
				Name:      "example.lan",
			},
		},
	)

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].FQDN != "self.example.lan" {
		t.Fatalf("expected full fqdn, got %q", records[0].FQDN)
	}
	if records[0].TargetDeviceID != "device-1" {
		t.Fatalf("expected target device id, got %q", records[0].TargetDeviceID)
	}
	if records[0].Value != "device-1" {
		t.Fatalf("expected raw value, got %q", records[0].Value)
	}
}

func TestNetworkEventDNSRecordsPreservesTxtAndSrvValues(t *testing.T) {
	records := networkEventDNSRecords(
		[]model.DNSRecord{
			{
				RecordID:  "rec-txt",
				NetworkID: "net-1",
				ZoneID:    "zone-1",
				Name:      "txt",
				Type:      "TXT",
				Value:     "hello-slan",
				TTL:       60,
			},
			{
				RecordID:  "rec-srv",
				NetworkID: "net-1",
				ZoneID:    "zone-1",
				Name:      "_sip._tcp",
				Type:      "SRV",
				Value:     "peer.example.lan",
				Port:      "5060",
				TTL:       60,
			},
		},
		[]model.DNSZone{
			{
				ZoneID:    "zone-1",
				NetworkID: "net-1",
				Name:      "example.lan",
			},
		},
	)

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Value != "hello-slan" {
		t.Fatalf("expected TXT value, got %q", records[0].Value)
	}
	if records[1].Value != "peer.example.lan" {
		t.Fatalf("expected SRV value, got %q", records[1].Value)
	}
	if records[1].Port != 5060 {
		t.Fatalf("expected SRV port 5060, got %d", records[1].Port)
	}
}

func TestBuildNetworkEventSnapshotPayloadBuildsFullFQDN(t *testing.T) {
	payload := buildNetworkEventSnapshotPayload(NetworkResolvedConfigView{
		Config: NetworkConfigView{
			Network: NetworkView{
				NetworkID: "net-1",
				Name:      "Default",
			},
			DeviceID:   "device-local",
			GlobalIP:   "10.0.1.10",
			GlobalName: "local",
			DNSZones: []DNSZoneView{
				{
					ZoneID:    "zone-1",
					NetworkID: "net-1",
					Name:      "example.lan",
				},
			},
			DNSRecords: []DNSRecordView{
				{
					RecordID:       "rec-1",
					NetworkID:      "net-1",
					ZoneID:         "zone-1",
					Name:           "self",
					Type:           "A",
					TargetDeviceID: "device-local",
					TTL:            60,
				},
			},
		},
	})

	if len(payload.DNSZones) != 1 {
		t.Fatalf("expected 1 dns zone, got %d", len(payload.DNSZones))
	}
	if len(payload.DNSRecords) != 1 {
		t.Fatalf("expected 1 dns record, got %d", len(payload.DNSRecords))
	}
	if payload.DNSRecords[0].FQDN != "self.example.lan" {
		t.Fatalf("expected full fqdn, got %q", payload.DNSRecords[0].FQDN)
	}
}
