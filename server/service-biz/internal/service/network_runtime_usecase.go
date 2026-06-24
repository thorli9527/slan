package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/repository"
)

func relayCandidates(ctx context.Context, networks repository.NetworkRepository, ops repository.OpsRepository, nowFn func() time.Time, networkID string) ([]RelayCandidateView, error) {
	items, err := listRelayNodeEntities(ctx, networks, ops, nowFn, networkID)
	if err != nil {
		return nil, err
	}
	return relayCandidatesFromNodes(items), nil
}

func (s NetworkRuntimeService) RelayCandidates(ctx context.Context, input RelayCandidatesInput) ([]RelayCandidateView, error) {
	input = normalizeRelayCandidatesInput(input)
	if input.NetworkID == "" || input.DeviceID == "" {
		return []RelayCandidateView{}, ErrInvalidArgument
	}
	items, err := relayCandidates(ctx, s.Networks, s.Ops, s.Now, input.NetworkID)
	if err != nil {
		return nil, err
	}
	runtimePath, ok, err := runtimePathForDevice(ctx, s.Networks, input.NetworkID, input.DeviceID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return items, nil
	}
	items = orderRelayCandidatesByRuntime(runtimePath, items)
	for i := range items {
		applyRuntimeSelection(&items[i], runtimePath)
	}
	return items, nil
}

func (s NetworkRuntimeService) ListPunchNodes(ctx context.Context) ([]PunchNodeView, error) {
	items, err := listPunchNodeEntities(ctx, s.Ops, s.Now)
	if err != nil {
		return nil, err
	}
	return punchNodeViews(items), nil
}

func (s NetworkRuntimeService) CreatePunchConnectSession(ctx context.Context, input CreatePunchConnectSessionInput) (PunchConnectSessionView, error) {
	input = normalizeCreatePunchConnectSessionInput(input)
	if input.NetworkID == "" || input.RequesterNodeID == "" || input.PeerNodeID == "" {
		return PunchConnectSessionView{}, ErrInvalidArgument
	}
	if _, err := requireManagedNetwork(ctx, s.Networks, input.NetworkID); err != nil {
		return PunchConnectSessionView{}, err
	}
	punchNodes, err := listPunchNodeEntities(ctx, s.Ops, s.Now)
	if err != nil {
		return PunchConnectSessionView{}, err
	}
	endpointAddress := "0.0.0.0:0"
	punchNodeID := ""
	if len(punchNodes) > 0 {
		endpointAddress = punchNodes[0].Endpoint
		punchNodeID = punchNodes[0].NodeID
	}
	item := newPunchConnectSession(newNetworkSessionID(s.NewSessID, "punch"), input, punchNodeID, endpointAddress)
	return punchConnectSessionView(item), nil
}

func (s NetworkRuntimeService) IssueRelayTicket(ctx context.Context, input IssueRelayTicketInput) (RelayTicketView, error) {
	input = normalizeIssueRelayTicketInput(input)
	if input.NetworkID == "" || input.SrcNodeID == "" || input.DstNodeID == "" {
		return RelayTicketView{}, ErrInvalidArgument
	}
	network, err := requireManagedNetwork(ctx, s.Networks, input.NetworkID)
	if err != nil {
		return RelayTicketView{}, err
	}
	if err := ensureRelayTicketAllowed(ctx, s.Devices, s.Networks, network, input.SrcNodeID, input.DstNodeID); err != nil {
		return RelayTicketView{}, err
	}
	relayNodes, err := listRelayNodeEntities(ctx, s.Networks, s.Ops, s.Now, input.NetworkID)
	if err != nil {
		return RelayTicketView{}, err
	}
	candidates := relayCandidatesFromNodes(relayNodes)
	runtimePath, ok, err := runtimePathForNode(ctx, s.Networks, input.NetworkID, input.SrcNodeID)
	if err != nil {
		return RelayTicketView{}, err
	}
	candidates, input.PreferredRelayEndpointIDs = prepareRelayTicketCandidates(candidates, input.PreferredRelayEndpointIDs, runtimePath, ok)
	sessionKey, err := randomHex(16)
	if err != nil {
		return RelayTicketView{}, err
	}
	signature, err := randomHex(24)
	if err != nil {
		return RelayTicketView{}, err
	}
	now := networkNow(s.Now).UTC()
	sessionID := newNetworkSessionID(s.NewSessID, "relay-session")
	candidate, found := chooseWireRelayCandidate(candidates, input.PreferredRelayEndpointIDs, sessionID)
	if !found {
		return RelayTicketView{}, ErrNotFound
	}
	if ok {
		applyRuntimeSelection(&candidate, runtimePath)
	}
	item := newRelayTicket(
		newNetworkSessionID(s.NewSessID, "relay-ticket"),
		sessionID,
		input,
		candidate,
		now.Add(10*time.Minute).Format(time.RFC3339Nano),
		sessionKey,
		signature,
	)
	return relayTicketView(item), nil
}
