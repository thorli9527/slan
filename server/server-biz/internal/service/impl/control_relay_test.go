package impl

import (
	"testing"

	"github.com/slan/server/server-biz/configs"
)

func TestRelayClustersForCountriesKeepsOnlyRequestedCountries(t *testing.T) {
	clusters := []relayClusterView{
		{countryCode: "CN", clusterID: "cn-a"},
		{countryCode: "US", clusterID: "us-a"},
		{countryCode: "SG", clusterID: "sg-a"},
	}

	filtered := relayClustersForCountries(clusters, []string{"us", " cn "})

	if len(filtered) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(filtered))
	}
	if filtered[0].clusterID != "cn-a" || filtered[1].clusterID != "us-a" {
		t.Fatalf("unexpected filtered clusters: %#v", filtered)
	}
}

func TestRelayRegionsFromClustersPreservesCountryAndEndpoints(t *testing.T) {
	regions := relayRegionsFromClusters([]relayClusterView{
		{
			countryCode: "CN",
			countryName: "China",
			cityCode:    "sha",
			cityName:    "Shanghai",
			clusterID:   "cn-a",
			clusterName: "CN A",
			nodes: []configs.RelayNodeConfig{
				{NodeID: "relay-cn-tcp", Transport: "tcp", Address: "127.0.0.1:9001"},
			},
		},
	})

	if len(regions) != 1 {
		t.Fatalf("expected one region, got %d", len(regions))
	}
	if regions[0].CountryCode != "CN" || regions[0].ClusterID != "cn-a" {
		t.Fatalf("unexpected region identity: %#v", regions[0])
	}
	if len(regions[0].Endpoints) != 1 || regions[0].Endpoints[0].EndpointID != "relay-cn-tcp" {
		t.Fatalf("unexpected endpoints: %#v", regions[0].Endpoints)
	}
}

func TestNormalizeRelayCountryCode(t *testing.T) {
	if got := normalizeRelayCountryCode(" cn "); got != "CN" {
		t.Fatalf("expected normalized country CN, got %q", got)
	}
	if got := normalizeRelayCountryCode(""); got != "" {
		t.Fatalf("expected empty country, got %q", got)
	}
}

func TestRelayHeartbeatRankPrefersHealthyThenUnknownThenUnhealthy(t *testing.T) {
	healthy := relayNodeRank{hasHeartbeat: true, heartbeatHealthy: true}
	unknown := relayNodeRank{}
	unhealthy := relayNodeRank{hasHeartbeat: true, heartbeatHealthy: false}

	if !betterRelayNodeRank(healthy, unknown) {
		t.Fatal("expected healthy heartbeat to outrank unknown heartbeat")
	}
	if !betterRelayNodeRank(unknown, unhealthy) {
		t.Fatal("expected unknown heartbeat to outrank unhealthy heartbeat")
	}
	if betterRelayNodeRank(unhealthy, healthy) {
		t.Fatal("expected unhealthy heartbeat to not outrank healthy heartbeat")
	}
}

func TestRelayPathOptionsPreserveClusterRankOrder(t *testing.T) {
	nodes := []configs.RelayNodeConfig{
		{NodeID: "healthy-low-static", Transport: "udp", Address: "127.0.0.1:9000", Priority: 100},
		{NodeID: "static-first", Transport: "udp", Address: "127.0.0.1:9001", Priority: 1},
	}

	_, preferred := relayPathOptions(nodes, "", "", 100)

	if len(preferred) != 2 {
		t.Fatalf("expected 2 preferred relay ids, got %d", len(preferred))
	}
	if preferred[0] != "healthy-low-static" {
		t.Fatalf("expected existing rank order to be preserved, got %#v", preferred)
	}
}
