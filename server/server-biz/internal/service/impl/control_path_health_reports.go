package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

// ReportPathHealth persists direct or relay path quality samples that later
// influence connect-plan sorting.
func (s dbControlChannelService) ReportPathHealth(userID, nodeID string, report controlmsg.PathHealthReport) error {
	pathType := strings.TrimSpace(report.PathType)
	derpNodeID := strings.TrimSpace(report.DerpNodeID)
	if strings.TrimSpace(report.NetworkID) == "" || pathType == "" {
		return fmt.Errorf("%w: networkId and pathType are required", ErrInvalidArgument)
	}
	if strings.TrimSpace(report.PeerNodeID) == "" && derpNodeID == "" {
		return fmt.Errorf("%w: peerNodeId or derpNodeId is required", ErrInvalidArgument)
	}

	ctx := context.Background()
	sourceNode, err := s.state.requireNodeSession(ctx, userID, nodeID, report.NetworkID)
	if err != nil {
		return err
	}
	peerNodeID := strings.TrimSpace(report.PeerNodeID)
	if peerNodeID != "" {
		peerNode, err := s.state.pg.GetNodeByID(ctx, peerNodeID)
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrNotFound
			}
			return err
		}
		if _, err := s.state.requireActiveNetworkMember(ctx, report.NetworkID, peerNode.DeviceID, ErrForbidden, "peer node device"); err != nil {
			return err
		}
		if _, err := s.state.requireActiveNetworkAttachment(ctx, report.NetworkID, peerNode.DeviceID, ErrForbidden, "peer node device"); err != nil {
			return err
		}
	}

	sampledAtMs := report.SampledAtMs
	if sampledAtMs == 0 {
		sampledAtMs = uint64(time.Now().UnixMilli())
	}

	now := time.Now()
	record := repo.NodePathHealth{
		HealthID:          util.NewID("path"),
		NetworkID:         report.NetworkID,
		NodeID:            sourceNode.NodeID,
		PeerNodeID:        peerNodeID,
		PathType:          pathType,
		ActivePath:        strings.TrimSpace(report.ActivePath),
		Endpoint:          strings.TrimSpace(report.Endpoint),
		DerpNodeID:        derpNodeID,
		ObservedRttMs:     report.ObservedRttMs,
		PacketLossPpm:     report.PacketLossPpm,
		PathScore:         report.PathScore,
		SourceCountryCode: strings.TrimSpace(report.SourceCountryCode),
		RelayCountryCode:  strings.TrimSpace(report.RelayCountryCode),
		PeerCountryCode:   strings.TrimSpace(report.PeerCountryCode),
		CrossCountry:      report.CrossCountry,
		RelayMtu:          report.RelayMtu,
		MaxFramePayload:   report.MaxFramePayload,
		PathDowngrades:    report.PathDowngrades,
		PathUpgrades:      report.PathUpgrades,
		LastPathChange:    strings.TrimSpace(report.LastPathChange),
		SampledAtMs:       sampledAtMs,
		UpdatedAt:         now.Unix(),
	}
	if err := s.state.pg.UpsertNodePathHealth(ctx, record); err != nil {
		return err
	}
	return s.state.pg.InsertNodePathHealthSample(ctx, repo.NodePathHealthSample{
		SampleID:          util.NewID("path-sample"),
		NetworkID:         record.NetworkID,
		NodeID:            record.NodeID,
		PeerNodeID:        record.PeerNodeID,
		PathType:          record.PathType,
		ActivePath:        record.ActivePath,
		Endpoint:          record.Endpoint,
		DerpNodeID:        record.DerpNodeID,
		ObservedRttMs:     record.ObservedRttMs,
		PacketLossPpm:     record.PacketLossPpm,
		PathScore:         record.PathScore,
		SourceCountryCode: record.SourceCountryCode,
		RelayCountryCode:  record.RelayCountryCode,
		PeerCountryCode:   record.PeerCountryCode,
		CrossCountry:      record.CrossCountry,
		RelayMtu:          record.RelayMtu,
		MaxFramePayload:   record.MaxFramePayload,
		PathDowngrades:    record.PathDowngrades,
		PathUpgrades:      record.PathUpgrades,
		LastPathChange:    record.LastPathChange,
		SampledAtMs:       record.SampledAtMs,
		UpdatedAt:         record.UpdatedAt,
	})
}
