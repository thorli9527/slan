package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
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
	network, err := requireManagedNetwork(ctx, s.Networks, input.NetworkID)
	if err != nil {
		return PunchConnectSessionView{}, err
	}
	if _, ok, err := runtimePathForNode(ctx, s.Networks, network.NetworkID, input.RequesterNodeID); err != nil {
		return PunchConnectSessionView{}, err
	} else if !ok {
		return PunchConnectSessionView{}, ErrNotFound
	}
	if _, ok, err := runtimePathForNode(ctx, s.Networks, network.NetworkID, input.PeerNodeID); err != nil {
		return PunchConnectSessionView{}, err
	} else if !ok {
		return PunchConnectSessionView{}, ErrNotFound
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
	now := networkNow(s.Now).UTC()
	sessionSeed := stableRelaySessionSeed(input.NetworkID, input.SrcNodeID, input.DstNodeID)
	candidate, found := chooseWireRelayCandidate(candidates, input.PreferredRelayEndpointIDs, sessionSeed)
	if !found {
		return RelayTicketView{}, ErrNotFound
	}
	if ok {
		applyRuntimeSelection(&candidate, runtimePath)
	}
	sessionID := stableRelaySessionID(input.NetworkID, input.SrcNodeID, input.DstNodeID, candidate)
	expiresAt := now.Add(10 * time.Minute).Format(time.RFC3339)
	ticketID := newNetworkSessionID(s.NewSessID, "relay-ticket")
	signature := signRelayBusinessTicket(
		ticketID,
		input.NetworkID,
		sessionID,
		input.SrcNodeID,
		input.DstNodeID,
		expiresAt,
	)
	item := newRelayTicket(
		ticketID,
		sessionID,
		input,
		candidate,
		expiresAt,
		sessionKey,
		signature,
	)
	return relayTicketView(item), nil
}

func signRelayBusinessTicket(
	ticketID string,
	networkID string,
	sessionID string,
	srcNodeID string,
	dstNodeID string,
	expiresAt string,
) string {
	secret := relayTicketSigningSecret()
	if secret == "" {
		return ""
	}
	payload := strings.Join([]string{
		strings.TrimSpace(ticketID),
		strings.TrimSpace(networkID),
		strings.TrimSpace(sessionID),
		strings.TrimSpace(srcNodeID),
		strings.TrimSpace(dstNodeID),
		strings.TrimSpace(expiresAt),
	}, "|")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func relayTicketSigningSecret() string {
	if secret := strings.TrimSpace(os.Getenv("SLAN_RELAY_TICKET_SECRET")); secret != "" {
		return secret
	}
	if secrets := parseRelayTicketSecretList(os.Getenv("SLAN_WIRE_TICKET_SECRETS")); len(secrets) > 0 {
		return secrets[0]
	}
	return strings.TrimSpace(os.Getenv("SLAN_WIRE_TICKET_SECRET"))
}

func parseRelayTicketSecretList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
