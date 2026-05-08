package impl

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
)

const (
	iceCandidateTTLSeconds  = 120
	iceRefreshAfterSeconds  = 300
	iceProbeIntervalSeconds = 60
)

type dbIceService struct{ state *dbState }

var _ service.Ice = dbIceService{}

func (s dbIceService) ListIceServers(region string, limit int) (dto.ClientIceServersResponse, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 7 {
		limit = 7
	}
	records, err := s.state.pg.ListIceServers(context.Background())
	if err != nil {
		return dto.ClientIceServersResponse{}, err
	}
	items := selectClientIceServers(records, strings.TrimSpace(region), limit)
	return dto.ClientIceServersResponse{
		IceServers:       items,
		RefreshAfterSec:  iceRefreshAfterSeconds,
		ProbeIntervalSec: iceProbeIntervalSeconds,
	}, nil
}

func (s dbIceService) ReportCandidates(userID, peerID string, req dto.ReportCandidatesRequest) (dto.ReportCandidatesResponse, error) {
	ctx := context.Background()
	peerID = util.FirstNonEmpty(peerID, req.PeerID)
	if strings.TrimSpace(peerID) == "" || len(req.Candidates) == 0 {
		return dto.ReportCandidatesResponse{}, service.ErrInvalidArgument
	}
	node, err := s.state.pg.GetNodeByID(ctx, peerID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.ReportCandidatesResponse{}, service.ErrNotFound
		}
		return dto.ReportCandidatesResponse{}, err
	}
	if node.UserID != userID {
		return dto.ReportCandidatesResponse{}, service.ErrForbidden
	}
	networkID := strings.TrimSpace(req.NetworkID)
	if networkID != "" {
		if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
			return dto.ReportCandidatesResponse{}, err
		}
	}
	now := time.Now().Unix()
	records := make([]repo.PeerCandidate, 0, len(req.Candidates))
	sourceServerIDs := make([]string, 0, len(req.Candidates))
	for _, item := range req.Candidates {
		candidateID := strings.TrimSpace(item.CandidateID)
		if candidateID == "" || strings.TrimSpace(item.CandidateType) == "" || strings.TrimSpace(item.Addr) == "" {
			continue
		}
		priority := item.Priority
		if priority <= 0 {
			priority = 100
		}
		records = append(records, repo.PeerCandidate{
			PeerID:         peerID,
			CandidateID:    candidateID,
			NetworkID:      networkID,
			CandidateType:  strings.TrimSpace(item.CandidateType),
			Addr:           strings.TrimSpace(item.Addr),
			SourceServerID: strings.TrimSpace(item.SourceServerID),
			Priority:       priority,
			UDPAvailable:   req.UDPAvailable,
			NATLevel:       strings.TrimSpace(req.NATLevel),
			MappingStable:  req.MappingStable,
			LastSeenAt:     now,
			CreatedAt:      now,
		})
		sourceServerIDs = append(sourceServerIDs, strings.TrimSpace(item.SourceServerID))
	}
	if len(records) == 0 {
		return dto.ReportCandidatesResponse{}, service.ErrInvalidArgument
	}
	if err := s.state.pg.UpsertPeerCandidates(ctx, records); err != nil {
		return dto.ReportCandidatesResponse{}, err
	}
	_ = s.state.pg.IncrementIceCandidateStats(ctx, sourceServerIDs)
	return dto.ReportCandidatesResponse{PeerID: peerID, Stored: len(records), ExpiresIn: iceCandidateTTLSeconds}, nil
}

func (s dbIceService) CreatePunchPlan(userID string, req dto.CreatePunchPlanRequest) (dto.PunchPlan, error) {
	ctx := context.Background()
	srcPeerID := util.FirstNonEmpty(req.SrcPeerID, req.PeerID)
	dstPeerID := strings.TrimSpace(req.DstPeerID)
	if dstPeerID == "" && strings.TrimSpace(req.PeerID) != "" {
		dstPeerID = strings.TrimSpace(req.PeerID)
	}
	if srcPeerID == "" || dstPeerID == "" || srcPeerID == dstPeerID {
		return dto.PunchPlan{}, service.ErrInvalidArgument
	}
	src, err := s.state.pg.GetNodeByID(ctx, srcPeerID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.PunchPlan{}, service.ErrNotFound
		}
		return dto.PunchPlan{}, err
	}
	if src.UserID != userID {
		return dto.PunchPlan{}, service.ErrForbidden
	}
	if _, err := s.state.pg.GetNodeByID(ctx, dstPeerID); err != nil {
		if repo.IsNotFound(err) {
			return dto.PunchPlan{}, service.ErrNotFound
		}
		return dto.PunchPlan{}, err
	}
	networkID := strings.TrimSpace(req.NetworkID)
	if networkID == "" {
		networkID, err = s.sharedNetworkForPeers(ctx, srcPeerID, dstPeerID)
		if err != nil {
			return dto.PunchPlan{}, err
		}
	}
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return dto.PunchPlan{}, err
	}
	cutoff := time.Now().Add(-time.Duration(iceCandidateTTLSeconds) * time.Second).Unix()
	aRecords, err := s.state.pg.ListFreshPeerCandidates(ctx, networkID, srcPeerID, cutoff)
	if err != nil {
		return dto.PunchPlan{}, err
	}
	bRecords, err := s.state.pg.ListFreshPeerCandidates(ctx, networkID, dstPeerID, cutoff)
	if err != nil {
		return dto.PunchPlan{}, err
	}
	now := time.Now().Unix()
	session := repo.PunchSession{
		SessionID: util.NewID("punch"),
		NetworkID: networkID,
		PeerA:     srcPeerID,
		PeerB:     dstPeerID,
		Status:    "created",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.state.pg.CreatePunchSession(ctx, session); err != nil {
		return dto.PunchPlan{}, err
	}
	return dto.PunchPlan{
		SessionID:     session.SessionID,
		NetworkID:     networkID,
		PeerA:         srcPeerID,
		PeerB:         dstPeerID,
		ACandidates:   candidateDTOs(aRecords),
		BCandidates:   candidateDTOs(bRecords),
		StartAfterMS:  300,
		TimeoutMS:     3000,
		RetryPolicy:   dto.PunchRetryPolicy{MaxRounds: 3, BurstMS: []int{0, 20, 40, 100, 120, 300}},
		FallbackRelay: s.fallbackRelay(),
	}, nil
}

func (s dbIceService) ReportPunchResult(userID, sessionID string, req dto.ReportPunchResultRequest) error {
	ctx := context.Background()
	sessionID = util.FirstNonEmpty(sessionID, req.SessionID)
	if sessionID == "" || strings.TrimSpace(req.ReporterPeerID) == "" || strings.TrimSpace(req.RemotePeerID) == "" {
		return service.ErrInvalidArgument
	}
	session, err := s.state.pg.GetPunchSession(ctx, sessionID)
	if err != nil {
		if repo.IsNotFound(err) {
			return service.ErrNotFound
		}
		return err
	}
	reporter, err := s.state.pg.GetNodeByID(ctx, req.ReporterPeerID)
	if err != nil {
		if repo.IsNotFound(err) {
			return service.ErrNotFound
		}
		return err
	}
	if reporter.UserID != userID {
		return service.ErrForbidden
	}
	if req.ReporterPeerID != session.PeerA && req.ReporterPeerID != session.PeerB {
		return service.ErrForbidden
	}
	now := time.Now().Unix()
	result := repo.PunchResult{
		ResultID:       util.NewID("punch-result"),
		SessionID:      sessionID,
		ReporterPeerID: strings.TrimSpace(req.ReporterPeerID),
		RemotePeerID:   strings.TrimSpace(req.RemotePeerID),
		Success:        req.Success,
		PathKind:       strings.TrimSpace(req.PathKind),
		ReportedAt:     now,
	}
	if req.SelectedPair != nil {
		result.LocalCandidateID = strings.TrimSpace(req.SelectedPair.LocalCandidateID)
		result.RemoteCandidateID = strings.TrimSpace(req.SelectedPair.RemoteCandidateID)
		result.LocalSrflxAddr = strings.TrimSpace(req.SelectedPair.LocalSrflxAddr)
		result.RemoteAddrSeen = strings.TrimSpace(req.SelectedPair.RemoteAddrSeen)
	}
	if req.Quality != nil {
		result.RTTMS = req.Quality.RTTMS
		result.LossRate = req.Quality.LossRate
		result.JitterMS = req.Quality.JitterMS
	}
	if req.Failure != nil {
		result.FailureReason = strings.TrimSpace(req.Failure.Reason)
		result.TriedPairs = req.Failure.TriedPairs
		result.TimeoutMS = req.Failure.TimeoutMS
	}
	if req.Fallback != nil {
		result.FallbackPath = strings.TrimSpace(req.Fallback.CurrentPath)
		result.FallbackRelayID = strings.TrimSpace(req.Fallback.RelayID)
	}
	if err := s.state.pg.InsertPunchResult(ctx, result); err != nil {
		return err
	}
	_ = s.updatePunchIceStats(ctx, result)
	status := "failed"
	if req.Success {
		status = "succeeded"
	}
	return s.state.pg.UpdatePunchSessionStatus(ctx, sessionID, status, now)
}

func (s dbIceService) ListOpsIceServers() ([]dto.IceServer, error) {
	records, err := s.state.pg.ListIceServers(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]dto.IceServer, 0, len(records))
	for _, record := range records {
		out = append(out, record.ToDTO())
	}
	return out, nil
}

func (s dbIceService) UpsertOpsIceServer(req dto.UpsertIceServerRequest) (dto.IceServer, error) {
	record, err := iceServerFromRequest(req, "")
	if err != nil {
		return dto.IceServer{}, err
	}
	if err := s.state.pg.UpsertIceServer(context.Background(), record); err != nil {
		return dto.IceServer{}, err
	}
	return record.ToDTO(), nil
}

func (s dbIceService) UpdateOpsIceServerStatus(serverID string, req dto.UpdateIceServerStatusRequest) (dto.IceServer, error) {
	status := normalizeIceServerStatus(req.Status)
	if status == "" {
		return dto.IceServer{}, service.ErrInvalidArgument
	}
	ctx := context.Background()
	if err := s.state.pg.UpdateIceServerStatus(ctx, strings.TrimSpace(serverID), status, time.Now().Unix()); err != nil {
		return dto.IceServer{}, err
	}
	record, err := s.state.pg.GetIceServer(ctx, serverID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.IceServer{}, service.ErrNotFound
		}
		return dto.IceServer{}, err
	}
	return record.ToDTO(), nil
}

func (s dbIceService) IceStats() (dto.IceStats, error) {
	records, err := s.state.pg.ListIceServerStats(context.Background())
	if err != nil {
		return dto.IceStats{}, err
	}
	out := make([]dto.IceServerStats, 0, len(records))
	for _, record := range records {
		out = append(out, record.ToDTO())
	}
	return dto.IceStats{Servers: out}, nil
}

func selectClientIceServers(records []repo.IceServer, region string, limit int) []dto.IceServer {
	filtered := make([]repo.IceServer, 0, len(records))
	for _, record := range records {
		switch record.Status {
		case "enabled", "degraded":
			filtered = append(filtered, record)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		if region != "" && a.Region != b.Region {
			if a.Region == region {
				return true
			}
			if b.Region == region {
				return false
			}
		}
		if a.Status != b.Status {
			return a.Status == "enabled"
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.Weight != b.Weight {
			return a.Weight > b.Weight
		}
		return a.ServerID < b.ServerID
	})
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	out := make([]dto.IceServer, 0, len(filtered))
	for _, record := range filtered {
		item := record.ToDTO()
		item.Name = ""
		item.PublicIP = ""
		item.Remark = ""
		item.CreatedAt = 0
		item.UpdatedAt = 0
		out = append(out, item)
	}
	return out
}

func (s *dbState) clientIceServersForNetworkMap(ctx context.Context, self dto.Node) []dto.IceServer {
	region := ""
	if device, err := s.pg.GetDeviceByID(ctx, self.DeviceID); err == nil {
		region = normalizeRelayCountryCode(device.CountryCode)
	}
	records, err := s.pg.ListIceServers(ctx)
	if err != nil {
		return nil
	}
	return selectClientIceServers(records, region, 5)
}

func iceServerFromRequest(req dto.UpsertIceServerRequest, fallbackID string) (repo.IceServer, error) {
	serverID := util.FirstNonEmpty(req.ServerID, fallbackID)
	if strings.TrimSpace(serverID) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Provider) == "" || strings.TrimSpace(req.Region) == "" || strings.TrimSpace(req.UDPAddr) == "" {
		return repo.IceServer{}, service.ErrInvalidArgument
	}
	stunPort := req.STUNPort
	if stunPort <= 0 {
		stunPort = 3478
	}
	priority := req.Priority
	if priority <= 0 {
		priority = 100
	}
	weight := req.Weight
	if weight <= 0 {
		weight = 100
	}
	status := normalizeIceServerStatus(req.Status)
	if status == "" {
		status = "enabled"
	}
	now := time.Now().Unix()
	return repo.IceServer{
		ServerID:  strings.TrimSpace(serverID),
		Name:      strings.TrimSpace(req.Name),
		Provider:  strings.TrimSpace(req.Provider),
		Region:    strings.TrimSpace(req.Region),
		Country:   util.FirstNonEmpty(req.Country, "CN"),
		PublicIP:  strings.TrimSpace(req.PublicIP),
		UDPAddr:   strings.TrimSpace(req.UDPAddr),
		STUNPort:  stunPort,
		Priority:  priority,
		Weight:    weight,
		Status:    status,
		Features:  strings.Join(util.DedupeTrimmed(req.Features), ","),
		Remark:    strings.TrimSpace(req.Remark),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func normalizeIceServerStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", "enabled":
		return "enabled"
	case "disabled", "degraded", "maintenance":
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func candidateDTOs(records []repo.PeerCandidate) []dto.Candidate {
	out := make([]dto.Candidate, 0, len(records))
	for _, record := range records {
		out = append(out, record.ToDTO())
	}
	return out
}

func (s dbIceService) sharedNetworkForPeers(ctx context.Context, srcPeerID, dstPeerID string) (string, error) {
	src, err := s.state.pg.GetNodeByID(ctx, srcPeerID)
	if err != nil {
		return "", err
	}
	dst, err := s.state.pg.GetNodeByID(ctx, dstPeerID)
	if err != nil {
		return "", err
	}
	srcMembers, err := s.state.pg.ListMembersByDevice(ctx, src.DeviceID)
	if err != nil {
		return "", err
	}
	dstNetworks := map[string]struct{}{}
	dstMembers, err := s.state.pg.ListMembersByDevice(ctx, dst.DeviceID)
	if err != nil {
		return "", err
	}
	for _, member := range dstMembers {
		if member.Status == "active" {
			dstNetworks[member.NetworkID] = struct{}{}
		}
	}
	for _, member := range srcMembers {
		if member.Status != "active" {
			continue
		}
		if _, ok := dstNetworks[member.NetworkID]; ok {
			return member.NetworkID, nil
		}
	}
	return "", fmt.Errorf("%w: no shared active network", service.ErrForbidden)
}

func (s dbIceService) fallbackRelay() dto.PunchFallbackRelay {
	for _, cluster := range s.state.relayClusters() {
		for _, node := range cluster.nodes {
			out := dto.PunchFallbackRelay{RelayID: node.NodeID}
			switch node.Transport {
			case "udp":
				out.UDPAddr = node.Address
			default:
				out.UDPAddr = node.Address
			}
			return out
		}
	}
	return dto.PunchFallbackRelay{}
}

func (s dbIceService) updatePunchIceStats(ctx context.Context, result repo.PunchResult) error {
	serverIDs := make([]string, 0, 2)
	if result.LocalCandidateID != "" {
		if candidate, err := s.state.pg.GetPeerCandidate(ctx, result.ReporterPeerID, result.LocalCandidateID); err == nil {
			serverIDs = append(serverIDs, candidate.SourceServerID)
		}
	}
	if result.RemoteCandidateID != "" {
		if candidate, err := s.state.pg.GetPeerCandidate(ctx, result.RemotePeerID, result.RemoteCandidateID); err == nil {
			serverIDs = append(serverIDs, candidate.SourceServerID)
		}
	}
	return s.state.pg.IncrementIcePunchStats(ctx, util.DedupeTrimmed(serverIDs), result.Success)
}
