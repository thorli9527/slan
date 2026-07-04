package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/slan/server/server-wire/internal/model"
	"github.com/slan/server/server-wire/internal/store"
)

type fakeBizAuthorizer struct {
	authz    model.PeerAuthzView
	runtime  model.PeerRuntimeConfigView
	topology model.NetworkTopologyView
	derpMap  model.DerpMap
	derpErr  error
	relays   []model.RelayNode
	relayErr error
}

func (f fakeBizAuthorizer) Enabled() bool { return true }

func (f fakeBizAuthorizer) PeerAuthz(context.Context, string) (model.PeerAuthzView, error) {
	return f.authz, nil
}

func (f fakeBizAuthorizer) PeerRuntimeConfig(context.Context, string) (model.PeerRuntimeConfigView, error) {
	if f.runtime.PeerID == "" {
		return model.PeerRuntimeConfigView{NetworkEnabled: true}, nil
	}
	return f.runtime, nil
}

func (f fakeBizAuthorizer) NetworkTopology(context.Context, string) (model.NetworkTopologyView, error) {
	return f.topology, nil
}

func (f fakeBizAuthorizer) DerpMap(context.Context) (model.DerpMap, error) {
	if f.derpErr != nil {
		return model.DerpMap{}, f.derpErr
	}
	return f.derpMap, nil
}

func (f fakeBizAuthorizer) RelayNodes(context.Context) ([]model.RelayNode, error) {
	if f.relayErr != nil {
		return nil, f.relayErr
	}
	return append([]model.RelayNode(nil), f.relays...), nil
}

type switchingBizAuthorizer struct {
	authz   model.PeerAuthzView
	derpMap model.DerpMap
	relays  []model.RelayNode
}

func (f *switchingBizAuthorizer) Enabled() bool { return true }

func (f *switchingBizAuthorizer) PeerAuthz(context.Context, string) (model.PeerAuthzView, error) {
	return f.authz, nil
}

func (f *switchingBizAuthorizer) PeerRuntimeConfig(context.Context, string) (model.PeerRuntimeConfigView, error) {
	return model.PeerRuntimeConfigView{NetworkEnabled: f.authz.Enabled}, nil
}

func (f *switchingBizAuthorizer) NetworkTopology(context.Context, string) (model.NetworkTopologyView, error) {
	return model.NetworkTopologyView{}, nil
}

func (f *switchingBizAuthorizer) DerpMap(context.Context) (model.DerpMap, error) {
	return f.derpMap, nil
}

func (f *switchingBizAuthorizer) RelayNodes(context.Context) ([]model.RelayNode, error) {
	return append([]model.RelayNode(nil), f.relays...), nil
}

type denyingBizAuthorizer struct {
	err error
}

func (f denyingBizAuthorizer) Enabled() bool { return true }

func (f denyingBizAuthorizer) PeerAuthz(context.Context, string) (model.PeerAuthzView, error) {
	if f.err != nil {
		return model.PeerAuthzView{}, f.err
	}
	return model.PeerAuthzView{PeerID: "other-peer", Enabled: true}, nil
}

func (f denyingBizAuthorizer) PeerRuntimeConfig(context.Context, string) (model.PeerRuntimeConfigView, error) {
	return model.PeerRuntimeConfigView{}, nil
}

func (f denyingBizAuthorizer) NetworkTopology(context.Context, string) (model.NetworkTopologyView, error) {
	return model.NetworkTopologyView{}, nil
}

func (f denyingBizAuthorizer) DerpMap(context.Context) (model.DerpMap, error) {
	return model.DerpMap{}, nil
}

func (f denyingBizAuthorizer) RelayNodes(context.Context) ([]model.RelayNode, error) {
	return nil, nil
}

func TestBuildPathPlanFromStoredPeer(t *testing.T) {
	svc := New(store.NewMemoryStore())
	_, err := svc.RegisterPeer(model.RegisterPeerRequest{
		Peer: model.PeerRegistration{
			PeerID:                "peer-a",
			NetworkID:             "net-a",
			SupportsLANDirect:     true,
			SupportsIPv6Direct:    true,
			SupportsDirectUDP:     true,
			SupportsRelayUDP:      true,
			SupportsDerpTCPTLS443: true,
			PreferLAN:             true,
			AllowFastReselection:  true,
		},
	})
	if err != nil {
		t.Fatalf("register peer: %v", err)
	}
	_, err = svc.ReportPathHealth(model.ReportPathHealthRequest{
		PeerID: "peer-a",
		Probes: []model.PathProbe{
			{Path: model.PathRelayUDP, Reachable: true, RTTMs: 40},
			{Path: model.PathDerpTCP443, Reachable: true, RTTMs: 90},
			{Path: model.PathDirectUDP, Reachable: true, RTTMs: 20},
			{Path: model.PathIPv6UDP, Reachable: true, RTTMs: 12},
			{Path: model.PathLANUDP, Reachable: true, RTTMs: 2},
		},
	})
	if err != nil {
		t.Fatalf("report path health: %v", err)
	}
	_, err = svc.ReportDerpHealth(model.ReportDerpHealthRequest{
		PeerID:  "peer-a",
		Samples: []model.DerpHealthSample{{RegionID: "cn-east", NodeID: "derp-cn-east-1", Reachable: true, RTTMs: 35}},
	})
	if err != nil {
		t.Fatalf("report derp health: %v", err)
	}

	plan, err := svc.BuildPathPlan(model.PathPlanRequest{PeerID: "peer-a"})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.PreferredPath != model.PathLANUDP {
		t.Fatalf("want lan_udp, got %s", plan.PreferredPath)
	}
	if len(plan.FallbackOrder) < 2 || plan.FallbackOrder[1] != model.PathIPv6UDP {
		t.Fatalf("unexpected fallback order: %#v", plan.FallbackOrder)
	}
}

func TestRegisterPeerRejectsBizDeniedPeer(t *testing.T) {
	t.Run("biz authz error", func(t *testing.T) {
		svc := NewWithBiz(store.NewMemoryStore(), denyingBizAuthorizer{err: errors.New("not found")})
		if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "missing-peer"}}); err == nil {
			t.Fatal("expected biz authz error to reject peer registration")
		}
	})

	t.Run("biz peer mismatch", func(t *testing.T) {
		svc := NewWithBiz(store.NewMemoryStore(), denyingBizAuthorizer{})
		if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "client-peer"}}); err == nil {
			t.Fatal("expected biz peer mismatch to reject peer registration")
		}
	})

	t.Run("biz disabled", func(t *testing.T) {
		svc := NewWithBiz(store.NewMemoryStore(), fakeBizAuthorizer{
			authz: model.PeerAuthzView{
				PeerID:  "disabled-peer",
				Enabled: false,
			},
		})
		if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "disabled-peer"}}); err == nil {
			t.Fatal("expected disabled biz peer to reject peer registration")
		}
	})
}

func TestRegisterPeerUsesBizAuthorization(t *testing.T) {
	svc := NewWithBiz(store.NewMemoryStore(), fakeBizAuthorizer{
		authz: model.PeerAuthzView{
			PeerID:     "peer-biz",
			NetworkID:  "net-from-biz",
			NodeID:     "node-from-biz",
			Enabled:    true,
			VirtualIPs: []string{"10.10.0.20"},
			AllowedIPs: []string{"10.10.0.20/32"},
		},
	})
	resp, err := svc.RegisterPeer(model.RegisterPeerRequest{
		Peer: model.PeerRegistration{
			PeerID:    "peer-biz",
			NetworkID: "client-net",
			NodeID:    "client-node",
		},
	})
	if err != nil {
		t.Fatalf("register peer: %v", err)
	}
	if resp.Peer.NetworkID != "net-from-biz" || resp.Peer.NodeID != "node-from-biz" {
		t.Fatalf("expected biz identity override, got %#v", resp.Peer)
	}
	if got := resp.Peer.VirtualIPs; len(got) != 1 || got[0] != "10.10.0.20" {
		t.Fatalf("expected biz virtual IPs, got %#v", got)
	}
}

func TestCriticalOperationsRecheckBizAuthorization(t *testing.T) {
	biz := &switchingBizAuthorizer{
		authz: model.PeerAuthzView{
			PeerID:     "peer-recheck",
			NetworkID:  "net-recheck",
			NodeID:     "node-recheck",
			Enabled:    true,
			VirtualIPs: []string{"10.10.0.21"},
			AllowedIPs: []string{"10.10.0.21/32"},
		},
	}
	svc := NewWithBiz(store.NewMemoryStore(), biz)
	if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "peer-recheck"}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}
	biz.authz.Enabled = false

	assertDenied := func(name string, fn func() error) {
		t.Helper()
		if err := fn(); err == nil {
			t.Fatalf("expected %s to require fresh biz authz", name)
		}
	}
	assertDenied("update endpoints", func() error {
		_, err := svc.UpdateEndpoints(model.UpdateEndpointsRequest{
			PeerID:    "peer-recheck",
			Endpoints: []model.Endpoint{{Kind: "udp", Address: "192.0.2.10", Port: 51820}},
		})
		return err
	})
	assertDenied("report path health", func() error {
		_, err := svc.ReportPathHealth(model.ReportPathHealthRequest{
			PeerID: "peer-recheck",
			Probes: []model.PathProbe{{Path: model.PathRelayUDP, Reachable: true}},
		})
		return err
	})
	assertDenied("report derp health", func() error {
		_, err := svc.ReportDerpHealth(model.ReportDerpHealthRequest{
			PeerID:  "peer-recheck",
			Samples: []model.DerpHealthSample{{RegionID: "cn-east", NodeID: "derp-cn-east-1", Reachable: true}},
		})
		return err
	})
	assertDenied("update active path", func() error {
		_, err := svc.UpdateActivePath(model.UpdateActivePathRequest{PeerID: "peer-recheck", Path: model.PathRelayUDP})
		return err
	})
	assertDenied("relay ticket", func() error {
		_, err := svc.IssueRelayTicket(model.IssueRelayTicketRequest{PeerID: "peer-recheck"})
		return err
	})
	assertDenied("derp ticket", func() error {
		_, err := svc.IssueDerpTicket(model.IssueDerpTicketRequest{PeerID: "peer-recheck"})
		return err
	})
	assertDenied("path plan", func() error {
		_, err := svc.BuildPathPlan(model.PathPlanRequest{PeerID: "peer-recheck"})
		return err
	})
	assertDenied("get peer", func() error {
		_, err := svc.GetPeer("peer-recheck")
		return err
	})
	assertDenied("runtime config", func() error {
		_, err := svc.GetPeerRuntimeConfig("peer-recheck")
		return err
	})
}

func TestRuntimeConfigUsesBizRuntimePolicy(t *testing.T) {
	svc := NewWithBiz(store.NewMemoryStore(), fakeBizAuthorizer{
		authz: model.PeerAuthzView{
			PeerID:     "peer-runtime",
			NetworkID:  "net-runtime",
			NodeID:     "node-runtime",
			Enabled:    true,
			VirtualIPs: []string{"10.10.0.30"},
			AllowedIPs: []string{"10.10.0.30/32"},
		},
		runtime: model.PeerRuntimeConfigView{
			PeerID:         "peer-runtime",
			NetworkEnabled: true,
			VirtualIPs:     []string{"10.10.0.31"},
			AllowedIPs:     []string{"10.10.0.31/32"},
		},
	})
	if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "peer-runtime", SupportsDirectUDP: true}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}
	cfg, err := svc.GetPeerRuntimeConfig("peer-runtime")
	if err != nil {
		t.Fatalf("runtime config: %v", err)
	}
	if len(cfg.VirtualIPs) != 1 || cfg.VirtualIPs[0] != "10.10.0.31" {
		t.Fatalf("expected runtime virtual IP override, got %#v", cfg.VirtualIPs)
	}
}

func TestBizDerpTicketsRequireBizDerpMap(t *testing.T) {
	biz := fakeBizAuthorizer{
		authz: model.PeerAuthzView{PeerID: "peer-derp-biz", NetworkID: "net-biz", NodeID: "node-biz", Enabled: true},
		derpMap: model.DerpMap{
			PreferredRegionID: "biz-region",
			Regions: []model.DerpRegion{{
				RegionID: "biz-region",
				Name:     "Biz Region",
				Nodes:    []model.DerpNode{{RegionID: "biz-region", NodeID: "derp-biz", Host: "derp-biz.local", Port: 443}},
			}, {
				RegionID: "backup-region",
				Name:     "Backup Region",
				Nodes:    []model.DerpNode{{RegionID: "backup-region", NodeID: "derp-backup", Host: "derp-backup.local", Port: 443}},
			}},
		},
	}
	svc := NewWithBiz(store.NewMemoryStore(), biz)
	if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "peer-derp-biz", SupportsDerpTCPTLS443: true}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}
	ticket, err := svc.IssueDerpTicket(model.IssueDerpTicketRequest{PeerID: "peer-derp-biz"})
	if err != nil {
		t.Fatalf("issue biz derp ticket: %v", err)
	}
	if ticket.Ticket.RegionID != "biz-region" || ticket.Ticket.NodeID != "derp-biz" {
		t.Fatalf("expected biz derp node, got %#v", ticket.Ticket)
	}
	if _, err := svc.IssueDerpTicket(model.IssueDerpTicketRequest{PeerID: "peer-derp-biz", RegionID: "cn-east", NodeID: "derp-cn-east-1"}); err == nil {
		t.Fatal("expected static derp node to be rejected when biz is enabled")
	}
	plan, err := svc.BuildPathPlan(model.PathPlanRequest{PeerID: "peer-derp-biz"})
	if err != nil {
		t.Fatalf("build path plan: %v", err)
	}
	if len(plan.DerpCandidates) != 2 || plan.DerpCandidates[0].NodeID != "derp-biz" || plan.DerpCandidates[1].NodeID != "derp-backup" {
		t.Fatalf("expected preferred derp first with backup candidates preserved, got %#v", plan.DerpCandidates)
	}
}

func TestBizRelayNodesGateRelayTicketAndPathPlan(t *testing.T) {
	biz := fakeBizAuthorizer{
		authz: model.PeerAuthzView{PeerID: "peer-relay-biz", NetworkID: "net-biz", NodeID: "node-biz", Enabled: true},
		derpMap: model.DerpMap{
			PreferredRegionID: "biz-region",
			Regions:           []model.DerpRegion{{RegionID: "biz-region", Nodes: []model.DerpNode{{RegionID: "biz-region", NodeID: "derp-biz", Host: "derp-biz.local", Port: 443}}}},
		},
		relays: []model.RelayNode{
			{RegionID: "biz-region", NodeID: "relay-biz", Host: "relay-biz.local", UDPPort: 29110, Enabled: true, Healthy: true, Priority: 10},
			{RegionID: "biz-region", NodeID: "relay-disabled", Host: "relay-disabled.local", UDPPort: 29110, Enabled: false, Healthy: true},
			{RegionID: "biz-region", NodeID: "relay-stale", Host: "relay-stale.local", UDPPort: 29110, Enabled: true, Healthy: true, Stale: true, Priority: 1},
		},
	}
	svc := NewWithBiz(store.NewMemoryStore(), biz)
	if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "peer-relay-biz", SupportsRelayUDP: true, SupportsDerpTCPTLS443: true}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}
	if _, err := svc.ReportPathHealth(model.ReportPathHealthRequest{PeerID: "peer-relay-biz", Probes: []model.PathProbe{{Path: model.PathRelayUDP, Reachable: true, RTTMs: 20}}}); err != nil {
		t.Fatalf("report path health: %v", err)
	}
	ticket, err := svc.IssueRelayTicket(model.IssueRelayTicketRequest{PeerID: "peer-relay-biz"})
	if err != nil {
		t.Fatalf("issue relay ticket: %v", err)
	}
	if ticket.Ticket.NodeID != "relay-biz" || ticket.Ticket.Host != "relay-biz.local" {
		t.Fatalf("expected relay ticket to use biz relay node, got %#v", ticket.Ticket)
	}
	plan, err := svc.BuildPathPlan(model.PathPlanRequest{PeerID: "peer-relay-biz"})
	if err != nil {
		t.Fatalf("build path plan: %v", err)
	}
	if len(plan.RelayCandidates) != 1 || plan.RelayCandidates[0].NodeID != "relay-biz" {
		t.Fatalf("expected only enabled healthy relay candidate, got %#v", plan.RelayCandidates)
	}
}

func TestBizRelayNodesRequiredForRelayPath(t *testing.T) {
	svc := NewWithBiz(store.NewMemoryStore(), fakeBizAuthorizer{
		authz: model.PeerAuthzView{PeerID: "peer-no-relay", NetworkID: "net-biz", NodeID: "node-biz", Enabled: true},
	})
	if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "peer-no-relay", SupportsRelayUDP: true}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}
	if _, err := svc.ReportPathHealth(model.ReportPathHealthRequest{PeerID: "peer-no-relay", Probes: []model.PathProbe{{Path: model.PathRelayUDP, Reachable: true, RTTMs: 20}}}); err != nil {
		t.Fatalf("report path health: %v", err)
	}
	if _, err := svc.IssueRelayTicket(model.IssueRelayTicketRequest{PeerID: "peer-no-relay"}); err == nil {
		t.Fatal("expected relay ticket to require biz relay node")
	} else if !errors.Is(err, ErrNoHealthyRelayNodes) {
		t.Fatalf("expected no healthy relay error, got %v", err)
	}
	plan, err := svc.BuildPathPlan(model.PathPlanRequest{PeerID: "peer-no-relay"})
	if err != nil {
		t.Fatalf("build path plan: %v", err)
	}
	for _, path := range plan.FallbackOrder {
		if path == model.PathRelayUDP {
			t.Fatalf("relay path should be removed without biz relay nodes: %#v", plan)
		}
	}
	if plan.DegradedReason != "relay_no_healthy_nodes" {
		t.Fatalf("expected relay degradation reason, got %#v", plan)
	}
}

func TestBizControlUnavailableUsesUnavailableDegradedReason(t *testing.T) {
	svc := NewWithBiz(store.NewMemoryStore(), fakeBizAuthorizer{
		authz:    model.PeerAuthzView{PeerID: "peer-control-down", NetworkID: "net-biz", NodeID: "node-biz", Enabled: true},
		relayErr: fmt.Errorf("biz relay endpoint down"),
		derpErr:  fmt.Errorf("biz derp endpoint down"),
	})
	if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "peer-control-down", SupportsDirectUDP: true, SupportsRelayUDP: true, SupportsDerpTCPTLS443: true}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}
	if _, err := svc.ReportPathHealth(model.ReportPathHealthRequest{PeerID: "peer-control-down", Probes: []model.PathProbe{
		{Path: model.PathDirectUDP, Reachable: true, RTTMs: 15},
		{Path: model.PathRelayUDP, Reachable: true, RTTMs: 20},
		{Path: model.PathDerpTCP443, Reachable: true, RTTMs: 80},
	}}); err != nil {
		t.Fatalf("report path health: %v", err)
	}
	plan, err := svc.BuildPathPlan(model.PathPlanRequest{PeerID: "peer-control-down"})
	if err != nil {
		t.Fatalf("build path plan: %v", err)
	}
	if plan.DegradedReason != "relay_control_unavailable,derp_control_unavailable" {
		t.Fatalf("unexpected degraded reason: %#v", plan)
	}
	if _, err := svc.GetDerpMap(); err == nil {
		t.Fatal("expected biz derp map read to fail when biz is unavailable")
	}
}

func TestBizPathPlanDegradesToDirectOnlyWithoutRelayAndDerp(t *testing.T) {
	svc := NewWithBiz(store.NewMemoryStore(), fakeBizAuthorizer{
		authz: model.PeerAuthzView{PeerID: "peer-direct-only", NetworkID: "net-biz", NodeID: "node-biz", Enabled: true},
	})
	if _, err := svc.RegisterPeer(model.RegisterPeerRequest{Peer: model.PeerRegistration{PeerID: "peer-direct-only", SupportsDirectUDP: true, SupportsRelayUDP: true, SupportsDerpTCPTLS443: true}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}
	if _, err := svc.ReportPathHealth(model.ReportPathHealthRequest{PeerID: "peer-direct-only", Probes: []model.PathProbe{
		{Path: model.PathDirectUDP, Reachable: true, RTTMs: 15},
		{Path: model.PathRelayUDP, Reachable: true, RTTMs: 20},
		{Path: model.PathDerpTCP443, Reachable: true, RTTMs: 80},
	}}); err != nil {
		t.Fatalf("report path health: %v", err)
	}
	plan, err := svc.BuildPathPlan(model.PathPlanRequest{PeerID: "peer-direct-only"})
	if err != nil {
		t.Fatalf("build path plan: %v", err)
	}
	if plan.PreferredPath != model.PathDirectUDP {
		t.Fatalf("expected direct-only degraded plan, got %#v", plan)
	}
	for _, path := range plan.FallbackOrder {
		if path == model.PathRelayUDP || path == model.PathDerpTCP443 {
			t.Fatalf("expected relay/derp removed from degraded plan: %#v", plan)
		}
	}
	if plan.DegradedReason != "relay_no_healthy_nodes,derp_no_healthy_nodes" {
		t.Fatalf("unexpected degraded reason: %#v", plan)
	}
}

func TestGetInternalViews(t *testing.T) {
	svc := New(store.NewMemoryStore())
	_, err := svc.RegisterPeer(model.RegisterPeerRequest{
		Peer: model.PeerRegistration{
			PeerID:                "peer-c",
			NetworkID:             "net-c",
			NodeID:                "node-c",
			VirtualIPs:            []string{"10.0.0.10"},
			AllowedIPs:            []string{"10.0.0.0/24"},
			SupportsDirectUDP:     true,
			SupportsRelayUDP:      true,
			SupportsDerpTCPTLS443: true,
			KeepaliveIntervalSecs: 15,
		},
	})
	if err != nil {
		t.Fatalf("register peer: %v", err)
	}
	_, err = svc.ReportPathHealth(model.ReportPathHealthRequest{
		PeerID: "peer-c",
		Probes: []model.PathProbe{{Path: model.PathDirectUDP, Reachable: true, RTTMs: 12}},
	})
	if err != nil {
		t.Fatalf("report path health: %v", err)
	}

	authz, err := svc.GetPeerAuthz("peer-c")
	if err != nil {
		t.Fatalf("get authz: %v", err)
	}
	if authz.NetworkID != "net-c" || !authz.Enabled {
		t.Fatalf("unexpected authz view: %#v", authz)
	}

	runtimeCfg, err := svc.GetPeerRuntimeConfig("peer-c")
	if err != nil {
		t.Fatalf("get runtime config: %v", err)
	}
	if runtimeCfg.PreferredPath != model.PathDirectUDP {
		t.Fatalf("unexpected preferred path: %s", runtimeCfg.PreferredPath)
	}

	topology, err := svc.GetNetworkTopology("net-c")
	if err != nil {
		t.Fatalf("get topology: %v", err)
	}
	if len(topology.Peers) != 1 || topology.Peers[0].PeerID != "peer-c" {
		t.Fatalf("unexpected topology: %#v", topology)
	}

	derpMap, err := svc.GetDerpMap()
	if err != nil {
		t.Fatalf("get derp map: %v", err)
	}
	if len(derpMap.Map.Regions) == 0 {
		t.Fatal("expected derp regions")
	}
	derpTicket, err := svc.IssueDerpTicket(model.IssueDerpTicketRequest{PeerID: "peer-c"})
	if err != nil {
		t.Fatalf("issue derp ticket: %v", err)
	}
	if derpTicket.Ticket.Path != model.PathDerpTCP443 {
		t.Fatalf("unexpected derp path: %s", derpTicket.Ticket.Path)
	}

	payload, err := json.Marshal(derpTicket.Ticket)
	if err != nil {
		t.Fatalf("marshal derp ticket: %v", err)
	}
	var contract struct {
		TicketID  string `json:"ticketId"`
		PeerID    string `json:"peerId"`
		NetworkID string `json:"networkId"`
		Path      string `json:"path"`
		RegionID  string `json:"regionId"`
		NodeID    string `json:"nodeId"`
	}
	if err := json.Unmarshal(payload, &contract); err != nil {
		t.Fatalf("unmarshal contract derp ticket: %v", err)
	}
	if contract.Path != "derp_tcp_tls_443" || contract.NodeID == "" || contract.RegionID == "" {
		t.Fatalf("unexpected derp ticket contract: %#v", contract)
	}

	relayTicket, err := svc.IssueRelayTicket(model.IssueRelayTicketRequest{PeerID: "peer-c"})
	if err != nil {
		t.Fatalf("issue relay ticket: %v", err)
	}
	if relayTicket.Ticket.SessionID == "" {
		t.Fatalf("expected relay ticket sessionId: %#v", relayTicket.Ticket)
	}
}

func TestUpdateActivePathTracksDowngrade(t *testing.T) {
	svc := New(store.NewMemoryStore())
	_, _ = svc.RegisterPeer(model.RegisterPeerRequest{
		Peer: model.PeerRegistration{
			PeerID:            "peer-b",
			SupportsDirectUDP: true,
			SupportsRelayUDP:  true,
		},
	})
	peer, err := svc.UpdateActivePath(model.UpdateActivePathRequest{
		PeerID: "peer-b",
		Path:   model.PathDirectUDP,
	})
	if err != nil {
		t.Fatalf("set direct path: %v", err)
	}
	if peer.ActivePath != model.PathDirectUDP {
		t.Fatalf("want direct path, got %s", peer.ActivePath)
	}
	peer, err = svc.UpdateActivePath(model.UpdateActivePathRequest{
		PeerID: "peer-b",
		Path:   model.PathRelayUDP,
	})
	if err != nil {
		t.Fatalf("set relay path: %v", err)
	}
	if peer.RecentPathDowngrades != 1 {
		t.Fatalf("want one downgrade, got %d", peer.RecentPathDowngrades)
	}
}
