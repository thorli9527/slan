package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbWireService) DerpMap() (dto.WireDerpMapView, error) {
	ctx := context.Background()
	freshAfterMs := s.state.wireNodeFreshAfterMs(time.Now())
	s.pruneStaleWireNodes(ctx)
	nodes, err := s.state.pg.ListWireDerpNodes(ctx, repo.WireNodeListOptions{
		SchedulableOnly: true,
		FreshAfterMs:    freshAfterMs,
	})
	if err != nil {
		return dto.WireDerpMapView{}, err
	}
	byRegion := make(map[string]int)
	out := dto.WireDerpMapView{Regions: make([]dto.WireDerpRegion, 0)}
	for _, node := range nodes {
		idx, ok := byRegion[node.RegionID]
		if !ok {
			if out.PreferredRegionID == "" {
				out.PreferredRegionID = node.RegionID
			}
			out.Regions = append(out.Regions, dto.WireDerpRegion{RegionID: node.RegionID, Name: node.Name})
			idx = len(out.Regions) - 1
			byRegion[node.RegionID] = idx
		}
		out.Regions[idx].Nodes = append(out.Regions[idx].Nodes, dto.WireDerpNode{
			RegionID: node.RegionID,
			NodeID:   node.NodeID,
			Host:     node.Host,
			Port:     node.Port,
		})
	}
	return out, nil
}

func (s dbWireService) ListDerpNodes() ([]dto.WireDerpNodeRecord, error) {
	ctx := context.Background()
	freshAfterMs := s.state.wireNodeFreshAfterMs(time.Now())
	s.pruneStaleWireNodes(ctx)
	nodes, err := s.state.pg.ListWireDerpNodes(ctx, repo.WireNodeListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]dto.WireDerpNodeRecord, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.ToDTOWithFreshness(freshAfterMs))
	}
	return out, nil
}

func (s dbWireService) UpsertDerpNode(req dto.UpsertWireDerpNodeRequest) (dto.WireDerpNodeRecord, error) {
	ctx := context.Background()
	record, err := wireDerpNodeFromRequest(req)
	if err != nil {
		return dto.WireDerpNodeRecord{}, err
	}
	before, beforeErr := s.state.pg.GetWireDerpNode(ctx, record.RegionID, record.NodeID)
	existed := beforeErr == nil
	if beforeErr != nil && !repo.IsNotFound(beforeErr) {
		return dto.WireDerpNodeRecord{}, beforeErr
	}
	if err := s.state.pg.UpsertWireDerpNode(ctx, record); err != nil {
		return dto.WireDerpNodeRecord{}, err
	}
	node, err := s.state.pg.GetWireDerpNode(ctx, record.RegionID, record.NodeID)
	if err != nil {
		return dto.WireDerpNodeRecord{}, err
	}
	s.state.recordWireNodeEvent(ctx, wireNodeEventFromDerpUpsert(before, node, existed))
	return node.ToDTO(), nil
}

func (s dbWireService) UpdateDerpNodeHealth(regionID, nodeID string, req dto.WireNodeHeartbeatRequest) (dto.WireDerpNodeRecord, error) {
	ctx := context.Background()
	regionID = strings.TrimSpace(regionID)
	nodeID = strings.TrimSpace(nodeID)
	if regionID == "" || nodeID == "" {
		return dto.WireDerpNodeRecord{}, ErrInvalidArgument
	}
	before, err := s.state.pg.GetWireDerpNode(ctx, regionID, nodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.WireDerpNodeRecord{}, ErrNotFound
		}
		return dto.WireDerpNodeRecord{}, err
	}
	if err := s.state.pg.UpdateWireDerpNodeHealth(ctx, regionID, nodeID, req.Healthy, time.Now().UnixMilli(), req.TicketKeyRotation); err != nil {
		if repo.IsNotFound(err) {
			return dto.WireDerpNodeRecord{}, ErrNotFound
		}
		return dto.WireDerpNodeRecord{}, err
	}
	node, err := s.state.pg.GetWireDerpNode(ctx, regionID, nodeID)
	if err != nil {
		return dto.WireDerpNodeRecord{}, err
	}
	if before.Healthy != node.Healthy {
		s.state.recordWireNodeEvent(ctx, wireNodeHealthEvent("derp", node.RegionID, node.NodeID, "heartbeat", before.Enabled, node.Enabled, before.Healthy, node.Healthy, "heartbeat"))
	}
	return node.ToDTO(), nil
}

func (s dbWireService) UpdateDerpNodeStatus(regionID, nodeID string, req dto.UpdateWireNodeStatusRequest) (dto.WireDerpNodeRecord, error) {
	ctx := context.Background()
	regionID = strings.TrimSpace(regionID)
	nodeID = strings.TrimSpace(nodeID)
	if regionID == "" || nodeID == "" || (req.Enabled == nil && req.Healthy == nil) {
		return dto.WireDerpNodeRecord{}, ErrInvalidArgument
	}
	before, err := s.state.pg.GetWireDerpNode(ctx, regionID, nodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.WireDerpNodeRecord{}, ErrNotFound
		}
		return dto.WireDerpNodeRecord{}, err
	}
	updates := map[string]any{"updated_at_ms": time.Now().UnixMilli()}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Healthy != nil {
		updates["healthy"] = *req.Healthy
	}
	if err := s.state.pg.UpdateWireDerpNodeStatus(ctx, regionID, nodeID, updates); err != nil {
		if repo.IsNotFound(err) {
			return dto.WireDerpNodeRecord{}, ErrNotFound
		}
		return dto.WireDerpNodeRecord{}, err
	}
	node, err := s.state.pg.GetWireDerpNode(ctx, regionID, nodeID)
	if err != nil {
		return dto.WireDerpNodeRecord{}, err
	}
	if before.Enabled != node.Enabled || before.Healthy != node.Healthy {
		s.state.recordWireNodeEvent(ctx, wireNodeHealthEvent("derp", node.RegionID, node.NodeID, "status_changed", before.Enabled, node.Enabled, before.Healthy, node.Healthy, "admin_status"))
	}
	return node.ToDTO(), nil
}

func (s dbWireService) ListRelayNodes() ([]dto.WireRelayNodeRecord, error) {
	ctx := context.Background()
	freshAfterMs := s.state.wireNodeFreshAfterMs(time.Now())
	s.pruneStaleWireNodes(ctx)
	nodes, err := s.state.pg.ListWireRelayNodes(ctx, repo.WireNodeListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]dto.WireRelayNodeRecord, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.ToDTOWithFreshness(freshAfterMs))
	}
	return out, nil
}

func (s dbWireService) UpsertRelayNode(req dto.UpsertWireRelayNodeRequest) (dto.WireRelayNodeRecord, error) {
	ctx := context.Background()
	record, err := wireRelayNodeFromRequest(req)
	if err != nil {
		return dto.WireRelayNodeRecord{}, err
	}
	before, beforeErr := s.state.pg.GetWireRelayNode(ctx, record.RegionID, record.NodeID)
	existed := beforeErr == nil
	if beforeErr != nil && !repo.IsNotFound(beforeErr) {
		return dto.WireRelayNodeRecord{}, beforeErr
	}
	if err := s.state.pg.UpsertWireRelayNode(ctx, record); err != nil {
		return dto.WireRelayNodeRecord{}, err
	}
	node, err := s.state.pg.GetWireRelayNode(ctx, record.RegionID, record.NodeID)
	if err != nil {
		return dto.WireRelayNodeRecord{}, err
	}
	s.state.recordWireNodeEvent(ctx, wireNodeEventFromRelayUpsert(before, node, existed))
	return node.ToDTO(), nil
}

func (s dbWireService) UpdateRelayNodeHealth(regionID, nodeID string, req dto.WireNodeHeartbeatRequest) (dto.WireRelayNodeRecord, error) {
	ctx := context.Background()
	regionID = strings.TrimSpace(regionID)
	nodeID = strings.TrimSpace(nodeID)
	if regionID == "" || nodeID == "" {
		return dto.WireRelayNodeRecord{}, ErrInvalidArgument
	}
	before, err := s.state.pg.GetWireRelayNode(ctx, regionID, nodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.WireRelayNodeRecord{}, ErrNotFound
		}
		return dto.WireRelayNodeRecord{}, err
	}
	if err := s.state.pg.UpdateWireRelayNodeHealth(ctx, regionID, nodeID, req.Healthy, time.Now().UnixMilli(), req.TicketKeyRotation); err != nil {
		if repo.IsNotFound(err) {
			return dto.WireRelayNodeRecord{}, ErrNotFound
		}
		return dto.WireRelayNodeRecord{}, err
	}
	node, err := s.state.pg.GetWireRelayNode(ctx, regionID, nodeID)
	if err != nil {
		return dto.WireRelayNodeRecord{}, err
	}
	if before.Healthy != node.Healthy {
		s.state.recordWireNodeEvent(ctx, wireNodeHealthEvent("relay", node.RegionID, node.NodeID, "heartbeat", before.Enabled, node.Enabled, before.Healthy, node.Healthy, "heartbeat"))
	}
	return node.ToDTO(), nil
}

func (s dbWireService) UpdateRelayNodeStatus(regionID, nodeID string, req dto.UpdateWireNodeStatusRequest) (dto.WireRelayNodeRecord, error) {
	ctx := context.Background()
	regionID = strings.TrimSpace(regionID)
	nodeID = strings.TrimSpace(nodeID)
	if regionID == "" || nodeID == "" || (req.Enabled == nil && req.Healthy == nil) {
		return dto.WireRelayNodeRecord{}, ErrInvalidArgument
	}
	before, err := s.state.pg.GetWireRelayNode(ctx, regionID, nodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.WireRelayNodeRecord{}, ErrNotFound
		}
		return dto.WireRelayNodeRecord{}, err
	}
	updates := map[string]any{"updated_at_ms": time.Now().UnixMilli()}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Healthy != nil {
		updates["healthy"] = *req.Healthy
	}
	if err := s.state.pg.UpdateWireRelayNodeStatus(ctx, regionID, nodeID, updates); err != nil {
		if repo.IsNotFound(err) {
			return dto.WireRelayNodeRecord{}, ErrNotFound
		}
		return dto.WireRelayNodeRecord{}, err
	}
	node, err := s.state.pg.GetWireRelayNode(ctx, regionID, nodeID)
	if err != nil {
		return dto.WireRelayNodeRecord{}, err
	}
	if before.Enabled != node.Enabled || before.Healthy != node.Healthy {
		s.state.recordWireNodeEvent(ctx, wireNodeHealthEvent("relay", node.RegionID, node.NodeID, "status_changed", before.Enabled, node.Enabled, before.Healthy, node.Healthy, "admin_status"))
	}
	return node.ToDTO(), nil
}

func wireDerpNodeFromRequest(req dto.UpsertWireDerpNodeRequest) (repo.WireDerpNode, error) {
	regionID := strings.TrimSpace(req.RegionID)
	nodeID := strings.TrimSpace(req.NodeID)
	host := strings.TrimSpace(req.Host)
	if regionID == "" || nodeID == "" || host == "" {
		return repo.WireDerpNode{}, ErrInvalidArgument
	}
	port := req.Port
	if port <= 0 {
		port = 443
	}
	healthy := true
	if req.Healthy != nil {
		healthy = *req.Healthy
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	priority := req.Priority
	if priority <= 0 {
		priority = 100
	}
	return repo.WireDerpNode{
		RegionID:            regionID,
		NodeID:              nodeID,
		Name:                strings.TrimSpace(req.Name),
		Host:                host,
		Port:                port,
		Enabled:             enabled,
		Healthy:             healthy,
		Priority:            priority,
		UpdatedAtMs:         time.Now().UnixMilli(),
		TicketKeySource:     req.TicketKeyRotation.Source,
		TicketKeyRingID:     req.TicketKeyRotation.KeyRingID,
		TicketKeyCount:      wireTicketKeyCount(req.TicketKeyRotation),
		TicketRotationReady: req.TicketKeyRotation.RotationReady,
	}, nil
}

func wireRelayNodeFromRequest(req dto.UpsertWireRelayNodeRequest) (repo.WireRelayNode, error) {
	regionID := strings.TrimSpace(req.RegionID)
	nodeID := strings.TrimSpace(req.NodeID)
	host := strings.TrimSpace(req.Host)
	if regionID == "" || nodeID == "" || host == "" || req.UDPPort <= 0 {
		return repo.WireRelayNode{}, ErrInvalidArgument
	}
	healthy := true
	if req.Healthy != nil {
		healthy = *req.Healthy
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	priority := req.Priority
	if priority <= 0 {
		priority = 100
	}
	return repo.WireRelayNode{
		RegionID:            regionID,
		NodeID:              nodeID,
		Host:                host,
		UDPPort:             req.UDPPort,
		AdminPort:           req.AdminPort,
		Enabled:             enabled,
		Healthy:             healthy,
		Priority:            priority,
		UpdatedAtMs:         time.Now().UnixMilli(),
		TicketKeySource:     req.TicketKeyRotation.Source,
		TicketKeyRingID:     req.TicketKeyRotation.KeyRingID,
		TicketKeyCount:      wireTicketKeyCount(req.TicketKeyRotation),
		TicketRotationReady: req.TicketKeyRotation.RotationReady,
	}, nil
}

func (s dbWireService) WireNodesOps() (dto.WireNodesOpsView, error) {
	derp, err := s.ListDerpNodes()
	if err != nil {
		return dto.WireNodesOpsView{}, err
	}
	relay, err := s.ListRelayNodes()
	if err != nil {
		return dto.WireNodesOpsView{}, err
	}
	derpMap, err := s.DerpMap()
	if err != nil {
		return dto.WireNodesOpsView{}, err
	}
	events, err := s.state.recentWireNodeEvents(context.Background(), 50)
	if err != nil {
		return dto.WireNodesOpsView{}, err
	}
	return dto.WireNodesOpsView{
		DerpNodes:             derp,
		RelayNodes:            relay,
		RecentEvents:          events,
		DerpMap:               derpMap,
		SchedulableDerpCount:  schedulableDerpCount(derp),
		SchedulableRelayCount: schedulableRelayCount(relay),
		StaleCount:            staleWireNodeCount(derp, relay),
		TicketKeyRotation:     wireTicketKeyStatus(),
		TicketKeyHealth:       s.wireTicketKeyHealth(derp, relay),
	}, nil
}

func wireTicketKeyCount(status dto.WireTicketKeyStatus) int {
	if status.EffectiveKeyCount > 0 {
		return status.EffectiveKeyCount
	}
	return status.KeyRingSize
}

func (s dbWireService) pruneStaleWireNodes(ctx context.Context) {
	s.state.pruneStaleWireNodes(ctx, time.Now())
}

func (s dbWireService) wireTicketKeyHealth(derp []dto.WireDerpNodeRecord, relay []dto.WireRelayNodeRecord) dto.WireTicketKeyHealth {
	instances := make([]dto.WireTicketKeyInstance, 0, len(s.state.cfg.Wire.ControlPlaneURLs)+len(derp)+len(relay))
	nowMs := time.Now().UnixMilli()
	for _, url := range s.state.cfg.Wire.ControlPlaneURLs {
		status := fetchWireTicketKeyStatus(url)
		status.ObservedAtMs = nowMs
		instances = append(instances, dto.WireTicketKeyInstance{
			Kind:   "wire",
			URL:    strings.TrimSpace(url),
			Status: status,
		})
	}
	for _, node := range relay {
		if !node.Enabled || !node.Healthy || node.Stale {
			continue
		}
		status := node.TicketKeyRotation
		if status.ObservedAtMs == 0 {
			status.ObservedAtMs = node.UpdatedAtMs
		}
		instances = append(instances, dto.WireTicketKeyInstance{
			Kind:     "relay",
			RegionID: node.RegionID,
			NodeID:   node.NodeID,
			Status:   status,
		})
	}
	for _, node := range derp {
		if !node.Enabled || !node.Healthy || node.Stale {
			continue
		}
		status := node.TicketKeyRotation
		if status.ObservedAtMs == 0 {
			status.ObservedAtMs = node.UpdatedAtMs
		}
		instances = append(instances, dto.WireTicketKeyInstance{
			Kind:     "derp",
			RegionID: node.RegionID,
			NodeID:   node.NodeID,
			Status:   status,
		})
	}
	return summarizeWireTicketKeyHealth(instances)
}

func fetchWireTicketKeyStatus(baseURL string) dto.WireTicketKeyStatus {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return dto.WireTicketKeyStatus{Error: "empty_url"}
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/internal/wire/ticket-key-status")
	if err != nil {
		return dto.WireTicketKeyStatus{Error: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return dto.WireTicketKeyStatus{Error: fmt.Sprintf("status=%d", resp.StatusCode)}
	}
	var status dto.WireTicketKeyStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return dto.WireTicketKeyStatus{Error: err.Error()}
	}
	status.Available = status.KeyRingID != ""
	if status.KeyRingSize == 0 {
		status.KeyRingSize = status.EffectiveKeyCount
	}
	return status
}

func summarizeWireTicketKeyHealth(instances []dto.WireTicketKeyInstance) dto.WireTicketKeyHealth {
	health := dto.WireTicketKeyHealth{Instances: instances, RotationReady: true}
	for i := range health.Instances {
		status := health.Instances[i].Status
		if status.KeyRingID == "" {
			health.UnavailableCount++
			health.RotationReady = false
			continue
		}
		if health.BaselineKeyRingID == "" {
			health.BaselineKeyRingID = status.KeyRingID
		}
		if status.KeyRingID != health.BaselineKeyRingID {
			health.Instances[i].Drifted = true
			health.Drifted = true
		}
		if !status.RotationReady {
			health.RotationReady = false
		}
	}
	return health
}

func wireTicketKeyStatus() dto.WireTicketKeyStatus {
	signing := strings.TrimSpace(os.Getenv("SLAN_WIRE_TICKET_SECRET")) != ""
	keyRingSize := 0
	if value := os.Getenv("SLAN_WIRE_TICKET_SECRETS"); value != "" {
		for _, item := range strings.Split(value, ",") {
			if strings.TrimSpace(item) != "" {
				keyRingSize++
			}
		}
	}
	return dto.WireTicketKeyStatus{
		SigningConfigured: signing,
		KeyRingConfigured: keyRingSize > 0,
		KeyRingSize:       keyRingSize,
		RotationReady:     signing && keyRingSize >= 2,
	}
}

func schedulableDerpCount(nodes []dto.WireDerpNodeRecord) int {
	count := 0
	for _, node := range nodes {
		if node.Enabled && node.Healthy && !node.Stale && strings.TrimSpace(node.Host) != "" && node.Port > 0 {
			count++
		}
	}
	return count
}

func schedulableRelayCount(nodes []dto.WireRelayNodeRecord) int {
	count := 0
	for _, node := range nodes {
		if node.Enabled && node.Healthy && !node.Stale && strings.TrimSpace(node.Host) != "" && node.UDPPort > 0 {
			count++
		}
	}
	return count
}

func staleWireNodeCount(derp []dto.WireDerpNodeRecord, relay []dto.WireRelayNodeRecord) int {
	count := 0
	for _, node := range derp {
		if node.Stale {
			count++
		}
	}
	for _, node := range relay {
		if node.Stale {
			count++
		}
	}
	return count
}

func (s *dbState) wireNodeHeartbeatFreshnessWindow() time.Duration {
	return durationFromSeconds(s.cfg.Wire.NodeHeartbeatFreshnessSeconds, 2*time.Minute)
}

func (s *dbState) wireNodeCleanupInterval() time.Duration {
	return durationFromSeconds(s.cfg.Wire.NodeCleanupIntervalSeconds, time.Minute)
}

func (s *dbState) wireNodeEventRetentionWindow() time.Duration {
	if s.cfg.Wire.NodeEventRetentionSeconds <= 0 {
		return 0
	}
	return time.Duration(s.cfg.Wire.NodeEventRetentionSeconds) * time.Second
}

func durationFromSeconds(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func (s *dbState) wireNodeFreshAfterMs(now time.Time) int64 {
	return now.Add(-s.wireNodeHeartbeatFreshnessWindow()).UnixMilli()
}

func (s *dbState) pruneStaleWireNodes(ctx context.Context, now time.Time) {
	cutoff := s.wireNodeFreshAfterMs(now)
	derpNodes, _ := s.pg.ListStaleHealthyWireDerpNodes(ctx, cutoff)
	relayNodes, _ := s.pg.ListStaleHealthyWireRelayNodes(ctx, cutoff)
	_ = s.pg.MarkStaleWireDerpNodesUnhealthy(ctx, cutoff)
	_ = s.pg.MarkStaleWireRelayNodesUnhealthy(ctx, cutoff)
	for _, node := range derpNodes {
		s.recordWireNodeEvent(ctx, wireNodeHealthEvent("derp", node.RegionID, node.NodeID, "stale_marked", node.Enabled, node.Enabled, true, false, "heartbeat_stale"))
	}
	for _, node := range relayNodes {
		s.recordWireNodeEvent(ctx, wireNodeHealthEvent("relay", node.RegionID, node.NodeID, "stale_marked", node.Enabled, node.Enabled, true, false, "heartbeat_stale"))
	}
	s.pruneWireNodeEvents(ctx, now)
}

func (s *dbState) pruneWireNodeEvents(ctx context.Context, now time.Time) {
	retention := s.wireNodeEventRetentionWindow()
	if retention <= 0 {
		return
	}
	_ = s.pg.DeleteWireNodeEventsBefore(ctx, now.Add(-retention).UnixMilli())
}

func (s *dbState) startWireNodeCleanupLoop() {
	go func() {
		ticker := time.NewTicker(s.wireNodeCleanupInterval())
		defer ticker.Stop()

		for range ticker.C {
			s.pruneStaleWireNodes(context.Background(), time.Now())
		}
	}()
}

func (s *dbState) recordWireNodeEvent(ctx context.Context, event repo.WireNodeEvent) {
	if event.NodeKind == "" || event.RegionID == "" || event.NodeID == "" || event.EventType == "" {
		return
	}
	if event.CreatedAtMs == 0 {
		event.CreatedAtMs = time.Now().UnixMilli()
	}
	_ = s.pg.InsertWireNodeEvent(ctx, event)
}

func (s *dbState) recentWireNodeEvents(ctx context.Context, limit int) ([]dto.WireNodeEventRecord, error) {
	events, err := s.pg.ListRecentWireNodeEvents(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]dto.WireNodeEventRecord, 0, len(events))
	for _, event := range events {
		out = append(out, event.ToDTO())
	}
	return out, nil
}

func wireNodeEventFromDerpUpsert(before, after repo.WireDerpNode, existed bool) repo.WireNodeEvent {
	eventType := "registered"
	reason := "register"
	var fromEnabled, fromHealthy *bool
	if existed {
		eventType = "status_changed"
		reason = "register_update"
		fromEnabled = boolPtr(before.Enabled)
		fromHealthy = boolPtr(before.Healthy)
	}
	return repo.WireNodeEvent{
		NodeKind:    "derp",
		RegionID:    after.RegionID,
		NodeID:      after.NodeID,
		EventType:   eventType,
		FromEnabled: fromEnabled,
		ToEnabled:   boolPtr(after.Enabled),
		FromHealthy: fromHealthy,
		ToHealthy:   boolPtr(after.Healthy),
		Reason:      reason,
	}
}

func wireNodeEventFromRelayUpsert(before, after repo.WireRelayNode, existed bool) repo.WireNodeEvent {
	eventType := "registered"
	reason := "register"
	var fromEnabled, fromHealthy *bool
	if existed {
		eventType = "status_changed"
		reason = "register_update"
		fromEnabled = boolPtr(before.Enabled)
		fromHealthy = boolPtr(before.Healthy)
	}
	return repo.WireNodeEvent{
		NodeKind:    "relay",
		RegionID:    after.RegionID,
		NodeID:      after.NodeID,
		EventType:   eventType,
		FromEnabled: fromEnabled,
		ToEnabled:   boolPtr(after.Enabled),
		FromHealthy: fromHealthy,
		ToHealthy:   boolPtr(after.Healthy),
		Reason:      reason,
	}
}

func wireNodeHealthEvent(kind, regionID, nodeID, eventType string, fromEnabled, toEnabled, fromHealthy, toHealthy bool, reason string) repo.WireNodeEvent {
	return repo.WireNodeEvent{
		NodeKind:    kind,
		RegionID:    regionID,
		NodeID:      nodeID,
		EventType:   eventType,
		FromEnabled: boolPtr(fromEnabled),
		ToEnabled:   boolPtr(toEnabled),
		FromHealthy: boolPtr(fromHealthy),
		ToHealthy:   boolPtr(toHealthy),
		Reason:      reason,
	}
}

func boolPtr(value bool) *bool {
	return &value
}
