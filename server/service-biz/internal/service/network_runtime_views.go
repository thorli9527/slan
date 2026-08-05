package service

import "github.com/slan/service-biz/internal/model"

func relayCandidateView(item model.RelayNode) RelayCandidateView {
	return RelayCandidateView{
		EndpointID: item.NodeID,
		Transport:  wireCandidateTransport(item.Transport),
		Address:    wireCandidateAddress(item.Endpoint),
		RegionID:   item.Region,
		ClusterID:  item.Region,
		Priority:   item.Priority,
	}
}

func punchNodeView(item model.PunchNode) PunchNodeView {
	host, port := splitOpsEndpoint(item.Endpoint)
	return PunchNodeView{
		NodeID:        item.NodeID,
		Name:          item.Name,
		Address:       item.Endpoint,
		PublicUDPIP:   host,
		PublicUDPPort: port,
		Priority:      item.Priority,
	}
}

func punchConnectEndpointView(item *model.PunchEndpoint) *PunchConnectEndpointView {
	if item == nil {
		return nil
	}
	return &PunchConnectEndpointView{
		NetworkID:    item.NetworkID,
		NodeID:       item.NodeID,
		EndpointType: item.EndpointType,
		Address:      item.Address,
		Reflexive:    item.Reflexive,
		NATType:      item.NATType,
	}
}

func punchConnectSessionView(item model.PunchConnectSession) PunchConnectSessionView {
	return PunchConnectSessionView{
		SessionID:       item.SessionID,
		NetworkID:       item.NetworkID,
		RequesterNodeID: item.RequesterNodeID,
		PeerNodeID:      item.PeerNodeID,
		PunchNodeID:     item.PunchNodeID,
		Requester:       punchConnectEndpointView(item.Requester),
		Peer:            punchConnectEndpointView(item.Peer),
	}
}

func relayTicketView(item model.RelayTicket) RelayTicketView {
	return RelayTicketView{
		TicketID:           item.TicketID,
		NetworkID:          item.NetworkID,
		SessionID:          item.SessionID,
		SrcNodeID:          item.SrcNodeID,
		DstNodeID:          item.DstNodeID,
		DERPClusterID:      item.DERPClusterID,
		AllowedDERPNodeIDs: append([]string(nil), item.AllowedDERPNodeIDs...),
		RelayURL:           item.RelayURL,
		ExpiresAt:          item.ExpiresAt,
		SessionKey:         item.SessionKey,
		Signature:          item.Signature,
	}
}
