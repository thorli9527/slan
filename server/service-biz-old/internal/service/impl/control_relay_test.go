package impl

import (
	"testing"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/netpath"
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

func TestNormalizeRelayTransportValue(t *testing.T) {
	if got := netpath.NormalizeRelayTransport(" UDP "); got != "udp" {
		t.Fatalf("expected udp, got %q", got)
	}
	for _, value := range []string{"tcp", "tls", "http3", "quic"} {
		if got := netpath.NormalizeRelayTransport(value); got != "" {
			t.Fatalf("expected unsupported transport to be rejected, got %q", got)
		}
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

func TestDirectPathOptionsExposeDirectUdpPathType(t *testing.T) {
	paths, nextPriority := directPathOptions([]dto.Endpoint{
		{Type: "lan", Address: "192.168.1.10:42000"},
		{Type: "reflexive", Address: "203.0.113.9:42000"},
	}, nil)

	if len(paths) != 2 {
		t.Fatalf("expected 2 direct paths, got %d", len(paths))
	}
	if paths[0].PathType != netpath.PathLanUdp {
		t.Fatalf("expected lan path type %q, got %#v", netpath.PathLanUdp, paths[0])
	}
	if paths[1].PathType != netpath.PathDirectUdp {
		t.Fatalf("expected direct path type %q, got %#v", netpath.PathDirectUdp, paths[1])
	}
	if paths[0].Priority != 10 || paths[1].Priority != 30 || nextPriority != 30 {
		t.Fatalf("unexpected direct path priorities paths=%#v next=%d", paths, nextPriority)
	}
}

func TestRelayTicketPrimaryNodeFollowsPreferredOrder(t *testing.T) {
	nodes := []configs.RelayNodeConfig{
		{NodeID: "relay-udp", Transport: "udp", Address: "127.0.0.1:9000"},
		{NodeID: "relay-udp-2", Transport: "udp", Address: "127.0.0.1:9001"},
	}
	req := dto.RelayTicketRequest{
		PreferredDerpNodeIDs: normalizePreferredRelayNodeIDs([]string{"relay-udp-2", "relay-udp"}, nodes),
	}

	node := relayTicketPrimaryNode(req, nodes)

	if node.NodeID != "relay-udp-2" || node.Transport != "udp" {
		t.Fatalf("expected preferred udp node, got %#v", node)
	}
}

func TestNormalizePreferredRelayNodeIDsPreservesPreferenceOrder(t *testing.T) {
	nodes := []configs.RelayNodeConfig{
		{NodeID: "relay-a"},
		{NodeID: "relay-b"},
		{NodeID: "relay-c"},
	}

	got := normalizePreferredRelayNodeIDs([]string{"relay-c", "relay-a", "relay-c", "missing"}, nodes)

	if len(got) != 2 || got[0] != "relay-c" || got[1] != "relay-a" {
		t.Fatalf("unexpected preferred node order: %#v", got)
	}
}
