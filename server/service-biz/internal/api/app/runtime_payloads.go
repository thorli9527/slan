package app

import servicepkg "github.com/slan/service-biz/internal/service"

func relayCandidateViewPayload(item servicepkg.RelayCandidateView) map[string]any {
	return relayCandidateBasePayload(item)
}

func punchConnectSessionPayload(item servicepkg.PunchConnectSessionView) map[string]any {
	return map[string]any{
		"sessionId":       item.SessionID,
		"networkId":       item.NetworkID,
		"requesterNodeId": item.RequesterNodeID,
		"peerNodeId":      item.PeerNodeID,
		"punchNodeId":     item.PunchNodeID,
		"requester":       punchConnectEndpointPayload(item.Requester),
		"peer":            punchConnectEndpointPayload(item.Peer),
	}
}

func punchConnectEndpointPayload(item *servicepkg.PunchConnectEndpointView) any {
	if item == nil {
		return nil
	}
	return map[string]any{
		"networkId": item.NetworkID,
		"nodeId":    item.NodeID,
		"type":      item.EndpointType,
		"address":   item.Address,
		"reflexive": item.Reflexive,
		"natType":   item.NATType,
	}
}

func relayTicketPayload(item servicepkg.RelayTicketView) map[string]any {
	return map[string]any{
		"ticketId":           item.TicketID,
		"networkId":          item.NetworkID,
		"sessionId":          item.SessionID,
		"srcNodeId":          item.SrcNodeID,
		"dstNodeId":          item.DstNodeID,
		"derpClusterId":      item.DERPClusterID,
		"allowedDerpNodeIds": item.AllowedDERPNodeIDs,
		"relayUrl":           item.RelayURL,
		"expiresAt":          item.ExpiresAt,
		"sessionKey":         item.SessionKey,
		"signature":          item.Signature,
	}
}
