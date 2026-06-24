package service

import "context"

type NetworkRuntimeUseCase interface {
	RelayCandidates(ctx context.Context, input RelayCandidatesInput) ([]RelayCandidateView, error)
	ListPunchNodes(ctx context.Context) ([]PunchNodeView, error)
	CreatePunchConnectSession(ctx context.Context, input CreatePunchConnectSessionInput) (PunchConnectSessionView, error)
	IssueRelayTicket(ctx context.Context, input IssueRelayTicketInput) (RelayTicketView, error)
}
