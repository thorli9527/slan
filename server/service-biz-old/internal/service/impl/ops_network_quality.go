package impl

import (
	"context"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbOpsService) NetworkQuality() (dto.OpsNetworkQuality, error) {
	ctx := context.Background()
	cutoff := time.Now().Add(-nodePathHealthFreshnessWindow).Unix()
	records, err := s.state.pg.ListRecentNodePathHealth(ctx, cutoff)
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
	networks, err := s.state.pg.ListNetworks(ctx)
	if err != nil {
		return dto.OpsNetworkQuality{}, err
	}
	policyExecutions, err := s.state.pg.ListRecentRelayPolicyExecutions(ctx, cutoff)
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
	networkNameByID := make(map[string]string, len(networks))
	for _, network := range networks {
		networkNameByID[network.NetworkID] = network.Name
	}
	policyExecutionByNetworkDevice := latestRelayPolicyExecutionByNetworkDevice(policyExecutions)

	items := make([]dto.OpsNetworkQualityItem, 0, len(records))
	devicesWithSample := make(map[string]bool, len(records))
	for _, record := range records {
		item := opsNetworkQualityItemFromRecord(record, networkNameByID)
		if node, ok := nodeByID[record.NodeID]; ok {
			item.UserID = node.UserID
			item.UserEmail = emailByUserID[node.UserID]
			item.DeviceID = node.DeviceID
			devicesWithSample[record.NetworkID+"|"+node.DeviceID] = true
			if device, ok := deviceByID[node.DeviceID]; ok {
				item.DeviceName = device.Name
			}
			if execution, ok := policyExecutionByNetworkDevice[record.NetworkID+"|"+node.DeviceID]; ok {
				applyRelayPolicyExecutionToQualityItem(&item, execution)
			}
		}
		items = append(items, item)
	}
	for key, execution := range policyExecutionByNetworkDevice {
		if devicesWithSample[key] {
			continue
		}
		items = append(items, policyOnlyQualityItem(
			execution,
			networkNameByID[execution.NetworkID],
			deviceByID,
			emailByUserID,
		))
	}
	return dto.OpsNetworkQuality{Items: items, Summary: buildOpsNetworkQualitySummary(records, networkNameByID)}, nil
}

func latestRelayPolicyExecutionByNetworkDevice(executions []repo.RelayPolicyExecution) map[string]repo.RelayPolicyExecution {
	out := make(map[string]repo.RelayPolicyExecution, len(executions))
	for _, execution := range executions {
		key := execution.NetworkID + "|" + execution.DeviceID
		if current, ok := out[key]; !ok || execution.UpdatedAt > current.UpdatedAt {
			out[key] = execution
		}
	}
	return out
}

func opsNetworkQualityItemFromRecord(record repo.NodePathHealth, networkNameByID map[string]string) dto.OpsNetworkQualityItem {
	item := dto.OpsNetworkQualityItem{
		HealthID:          record.HealthID,
		NetworkID:         record.NetworkID,
		NetworkName:       networkNameByID[record.NetworkID],
		NodeID:            record.NodeID,
		PeerNodeID:        record.PeerNodeID,
		PathType:          record.PathType,
		ActivePath:        record.ActivePath,
		Endpoint:          record.Endpoint,
		DerpNodeID:        record.DerpNodeID,
		SourceCountryCode: record.SourceCountryCode,
		RelayCountryCode:  record.RelayCountryCode,
		PeerCountryCode:   record.PeerCountryCode,
		CrossCountry:      record.CrossCountry,
		TicketExpiresAt:   record.TicketExpiresAt,
		TicketExpiresInMs: record.TicketExpiresInMs,
		TicketRenewDue:    record.TicketRenewDue,
		PathDowngrades:    record.PathDowngrades,
		PathUpgrades:      record.PathUpgrades,
		LastPathChange:    record.LastPathChange,
		SampledAtMs:       record.SampledAtMs,
		UpdatedAt:         record.UpdatedAt,
	}
	if value := record.ObservedRttMs; value != nil {
		item.ObservedRttMs = *value
	}
	if value := record.PacketLossPpm; value != nil {
		item.PacketLossPpm = *value
	}
	if value := record.PathScore; value != nil {
		item.PathScore = *value
	}
	if value := record.RelayMtu; value != nil {
		item.RelayMtu = *value
	}
	if value := record.MaxFramePayload; value != nil {
		item.MaxFramePayload = *value
	}
	return item
}

func applyRelayPolicyExecutionToQualityItem(item *dto.OpsNetworkQualityItem, execution repo.RelayPolicyExecution) {
	item.PolicyID = execution.PolicyID
	item.PolicyTemplateID = execution.TemplateID
	item.PolicyTemplateName = execution.TemplateName
	item.PolicyScope = execution.Scope
	item.PolicyApplied = execution.Applied
	item.PolicyUpdatedAtMs = execution.PolicyUpdatedAtMs
	item.PolicyReportedAtMs = execution.ReportedAtMs
}

func policyOnlyQualityItem(
	execution repo.RelayPolicyExecution,
	networkName string,
	deviceByID map[string]repo.Device,
	emailByUserID map[string]string,
) dto.OpsNetworkQualityItem {
	item := dto.OpsNetworkQualityItem{
		HealthID:        execution.ExecutionID,
		NetworkID:       execution.NetworkID,
		NetworkName:     networkName,
		DeviceID:        execution.DeviceID,
		NodeID:          execution.NodeID,
		PathType:        "policy",
		RelayMtu:        execution.RelayMtu,
		MaxFramePayload: execution.MaxFramePayload,
		UpdatedAt:       execution.UpdatedAt,
	}
	if device, ok := deviceByID[execution.DeviceID]; ok {
		item.DeviceName = device.Name
		item.UserID = device.UserID
		item.UserEmail = emailByUserID[device.UserID]
	}
	applyRelayPolicyExecutionToQualityItem(&item, execution)
	return item
}
