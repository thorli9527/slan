package service

import "github.com/slan/server/server-biz/api/dto"

// Ice owns control-plane ICE server registry, peer candidates, and punch plans.
type Ice interface {
	ListIceServers(region string, limit int) (dto.ClientIceServersResponse, error)
	ReportCandidates(userID, peerID string, req dto.ReportCandidatesRequest) (dto.ReportCandidatesResponse, error)
	CreatePunchPlan(userID string, req dto.CreatePunchPlanRequest) (dto.PunchPlan, error)
	ReportPunchResult(userID, sessionID string, req dto.ReportPunchResultRequest) error

	ListOpsIceServers() ([]dto.IceServer, error)
	UpsertOpsIceServer(req dto.UpsertIceServerRequest) (dto.IceServer, error)
	UpdateOpsIceServerStatus(serverID string, req dto.UpdateIceServerStatusRequest) (dto.IceServer, error)
	IceStats() (dto.IceStats, error)
}
