package impl

import (
	"context"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNetworkService) NetworkQuality(userID, networkID string, hours int) (dto.OpsNetworkQuality, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.OpsNetworkQuality{}, ErrNotFound
		}
		return dto.OpsNetworkQuality{}, err
	}
	if record.OwnerUserID != userID {
		return dto.OpsNetworkQuality{}, ErrForbidden
	}
	if hours <= 0 {
		hours = 1
	}
	if hours > 24 {
		hours = 24
	}
	cutoffMs := uint64(time.Now().Add(-time.Duration(hours) * time.Hour).UnixMilli())
	samples, err := s.state.pg.ListNodePathHealthSamplesByNetwork(ctx, networkID, cutoffMs)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	nodes, err := s.state.pg.ListNodes(ctx)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	devices, err := s.state.pg.ListDevices(ctx)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	users, err := s.state.pg.ListUsers(ctx)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	policyExecutions, err := s.state.pg.ListRecentRelayPolicyExecutions(ctx, time.Now().Add(-24*time.Hour).Unix())
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}

	nodeByID := make(map[string]repo.Node, len(nodes))
	for _, node := range nodes {
		nodeByID[node.NodeID] = node
	}
	deviceByID := make(map[string]repo.Device, len(devices))
	for _, device := range devices {
		deviceByID[device.DeviceID] = device
	}
	emailByUserID := make(map[string]string, len(users))
	for _, user := range users {
		emailByUserID[user.UserID] = user.Email
	}
	policyExecutionByDevice := make(map[string]repo.RelayPolicyExecution, len(policyExecutions))
	for _, execution := range policyExecutions {
		if execution.NetworkID != networkID {
			continue
		}
		if current, ok := policyExecutionByDevice[execution.DeviceID]; !ok || execution.UpdatedAt > current.UpdatedAt {
			policyExecutionByDevice[execution.DeviceID] = execution
		}
	}

	type aggregate struct {
		latest repo.NodePathHealthSample
		count  int
		rtt    qualityAccumulator
	}
	byDevice := make(map[string]*aggregate)
	latestRecords := make([]repo.NodePathHealth, 0, len(samples))
	for _, sample := range samples {
		node, ok := nodeByID[sample.NodeID]
		if !ok {
			continue
		}
		agg := byDevice[node.DeviceID]
		if agg == nil {
			agg = &aggregate{}
			byDevice[node.DeviceID] = agg
		}
		agg.count++
		agg.rtt.add(nodePathHealthFromSample(sample))
		if sample.SampledAtMs >= agg.latest.SampledAtMs {
			agg.latest = sample
		}
		latestRecords = append(latestRecords, nodePathHealthFromSample(sample))
	}

	items := make([]dto.OpsNetworkQualityItem, 0, len(byDevice))
	for deviceID, agg := range byDevice {
		sample := agg.latest
		node := nodeByID[sample.NodeID]
		item := dto.OpsNetworkQualityItem{
			HealthID:          sample.SampleID,
			NetworkID:         sample.NetworkID,
			NetworkName:       record.Name,
			UserID:            node.UserID,
			UserEmail:         emailByUserID[node.UserID],
			DeviceID:          deviceID,
			NodeID:            sample.NodeID,
			PeerNodeID:        sample.PeerNodeID,
			PathType:          sample.PathType,
			ActivePath:        sample.ActivePath,
			Endpoint:          sample.Endpoint,
			DerpNodeID:        sample.DerpNodeID,
			SourceCountryCode: sample.SourceCountryCode,
			RelayCountryCode:  sample.RelayCountryCode,
			PeerCountryCode:   sample.PeerCountryCode,
			CrossCountry:      sample.CrossCountry,
			PathDowngrades:    sample.PathDowngrades,
			PathUpgrades:      sample.PathUpgrades,
			LastPathChange:    sample.LastPathChange,
			SampledAtMs:       sample.SampledAtMs,
			UpdatedAt:         sample.UpdatedAt,
		}
		counter := agg.rtt.dto()
		item.ObservedRttMs = counter.AvgRttMs
		item.PacketLossPpm = counter.AvgPacketLossPpm
		item.PathScore = counter.AvgPathScore
		if sample.RelayMtu != nil {
			item.RelayMtu = *sample.RelayMtu
		}
		if sample.MaxFramePayload != nil {
			item.MaxFramePayload = *sample.MaxFramePayload
		}
		if device, ok := deviceByID[deviceID]; ok {
			item.DeviceName = device.Name
		}
		if execution, ok := policyExecutionByDevice[deviceID]; ok {
			item.PolicyID = execution.PolicyID
			item.PolicyScope = execution.Scope
			item.PolicyApplied = execution.Applied
			item.PolicyReportedAtMs = execution.ReportedAtMs
		}
		items = append(items, item)
	}
	return dto.OpsNetworkQuality{
		Items:   items,
		Summary: buildOpsNetworkQualitySummary(latestRecords, map[string]string{networkID: record.Name}),
	}, nil
}

func nodePathHealthFromSample(sample repo.NodePathHealthSample) repo.NodePathHealth {
	return repo.NodePathHealth{
		HealthID:          sample.SampleID,
		NetworkID:         sample.NetworkID,
		NodeID:            sample.NodeID,
		PeerNodeID:        sample.PeerNodeID,
		PathType:          sample.PathType,
		ActivePath:        sample.ActivePath,
		Endpoint:          sample.Endpoint,
		DerpNodeID:        sample.DerpNodeID,
		ObservedRttMs:     sample.ObservedRttMs,
		PacketLossPpm:     sample.PacketLossPpm,
		PathScore:         sample.PathScore,
		SourceCountryCode: sample.SourceCountryCode,
		RelayCountryCode:  sample.RelayCountryCode,
		PeerCountryCode:   sample.PeerCountryCode,
		CrossCountry:      sample.CrossCountry,
		RelayMtu:          sample.RelayMtu,
		MaxFramePayload:   sample.MaxFramePayload,
		TicketExpiresAt:   sample.TicketExpiresAt,
		TicketExpiresInMs: sample.TicketExpiresInMs,
		TicketRenewDue:    sample.TicketRenewDue,
		PathDowngrades:    sample.PathDowngrades,
		PathUpgrades:      sample.PathUpgrades,
		LastPathChange:    sample.LastPathChange,
		SampledAtMs:       sample.SampledAtMs,
		UpdatedAt:         sample.UpdatedAt,
	}
}
