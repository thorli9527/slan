package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/netpath"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s dbControlChannelService) ReportRelayHeartbeat(report controlmsg.RelayNodeHeartbeat) error {
	nodeID := strings.TrimSpace(report.NodeID)
	if nodeID == "" {
		return fmt.Errorf("%w: nodeId is required", ErrInvalidArgument)
	}
	transport := netpath.NormalizeRelayTransport(report.Transport)
	if strings.TrimSpace(report.Transport) != "" && transport == "" {
		return fmt.Errorf("%w: unsupported relay transport", ErrInvalidArgument)
	}
	reportedAtMs := report.ReportedAtMs
	if reportedAtMs == 0 {
		reportedAtMs = uint64(time.Now().UnixMilli())
	}
	return s.state.pg.UpsertRelayNodeHeartbeat(context.Background(), repo.RelayNodeHeartbeat{
		NodeID:         nodeID,
		ClusterID:      strings.TrimSpace(report.ClusterID),
		CountryCode:    normalizeRelayCountryCode(report.CountryCode),
		CityCode:       strings.TrimSpace(report.CityCode),
		Transport:      transport,
		Address:        strings.TrimSpace(report.Address),
		Healthy:        report.Healthy,
		ActiveSessions: report.ActiveSessions,
		ReportedAtMs:   reportedAtMs,
		UpdatedAt:      time.Now().Unix(),
	})
}

func (s dbControlChannelService) ReportRelayPolicy(userID, nodeID string, report controlmsg.RelayPolicyReport) error {
	networkID := strings.TrimSpace(report.NetworkID)
	if networkID == "" {
		return fmt.Errorf("%w: networkId is required", ErrInvalidArgument)
	}
	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID)
	if err != nil {
		return err
	}
	deviceID := strings.TrimSpace(report.DeviceID)
	if deviceID == "" {
		deviceID = node.DeviceID
	}
	if deviceID != node.DeviceID {
		return ErrForbidden
	}
	reportedAtMs := report.ReportedAtMS
	if reportedAtMs == 0 {
		reportedAtMs = uint64(time.Now().UnixMilli())
	}
	return s.state.pg.UpsertRelayPolicyExecution(ctx, repo.RelayPolicyExecution{
		ExecutionID:       util.NewID("relay-exec"),
		NetworkID:         networkID,
		DeviceID:          deviceID,
		NodeID:            node.NodeID,
		PolicyID:          strings.TrimSpace(report.PolicyID),
		Scope:             strings.TrimSpace(report.Scope),
		RelayMtu:          report.RelayMtu,
		MaxFramePayload:   report.MaxFramePayload,
		ExecutionLevel:    report.ExecutionLevel,
		Applied:           report.Applied,
		Reason:            strings.TrimSpace(report.Reason),
		PolicyUpdatedAtMs: report.PolicyUpdatedAtMS,
		ReportedAtMs:      reportedAtMs,
		UpdatedAt:         time.Now().Unix(),
	})
}
